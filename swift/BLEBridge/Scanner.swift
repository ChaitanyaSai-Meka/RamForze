import Foundation
import CoreBluetooth
import Network

final class ScannerBLEScanner: NSObject, CBCentralManagerDelegate {
    
    private var centralManager: CBCentralManager!
    
    private let serviceUUID = CBUUID(string: "8530AD31-BC8A-4A39-82E2-787A106F0F25")
    
    private var activeWorkers: [String: Date] = [:]
    private var cleanupTimer: Timer?
    
    private var socketConnection: NWConnection?
    
    override init() {
        super.init()
        
        setupSocket()
        
        centralManager = CBCentralManager(delegate: self, queue: nil)
        
        cleanupTimer = Timer.scheduledTimer(withTimeInterval: 5.0, repeats: true) { [weak self] _ in
            self?.sweepDeadWorkers()
        }
    }
    
    private func setupSocket() {
        let socketURL = URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent(".ramforze/ble.sock")
        
        socketConnection = NWConnection(to: .unix(path: socketURL.path), using: .tcp)
        socketConnection?.stateUpdateHandler = { state in
            switch state {
            case .ready:
                print("Scanner securely connected to Go backend via socket.")
            case .failed(let error):
                print("Socket connection failed: \(error). (Is the Go backend running?)")
            default:
                break
            }
        }
        socketConnection?.start(queue: .main)
    }
    
    func centralManagerDidUpdateState(_ central: CBCentralManager) {
        if central.state == .poweredOn {
            print("Master BLE ready. Scanning for Ramforze Workers...")
            
            centralManager.scanForPeripherals(
                withServices: [serviceUUID],
                options: [CBCentralManagerScanOptionAllowDuplicatesKey: true]
            )
        } else {
            print("Master BLE unavailable. State: \(central.state.rawValue)")
        }
    }
    
    func centralManager(_ central: CBCentralManager, didDiscover peripheral: CBPeripheral, advertisementData: [String : Any], rssi RSSI: NSNumber) {
        
        guard let payload = advertisementData[CBAdvertisementDataLocalNameKey] as? String else { return }
        let parts = payload.split(separator: "|").map { String($0) }
        
        if parts.count == 3 {
            let hostname = parts[0]
            let ipAddress = parts[1]
            let port = parts[2]
            
            if activeWorkers[ipAddress] == nil {
                print("➕ NEW WORKER FOUND: \(hostname) (\(ipAddress):\(port))")
                streamNDJSON(action: "add", ip: ipAddress, port: port, name: hostname)
            }
            
            activeWorkers[ipAddress] = Date()
        }
    }
    
    private func sweepDeadWorkers() {
        let now = Date()
        
        for (ipAddress, lastSeen) in activeWorkers {
            if now.timeIntervalSince(lastSeen) > 15.0 {
                print("➖ WORKER LOST: \(ipAddress) (Timeout)")
                
                activeWorkers.removeValue(forKey: ipAddress)
                streamNDJSON(action: "remove", ip: ipAddress)
            }
        }
    }
    
    private func streamNDJSON(action: String, ip: String, port: String = "", name: String = "") {
        let jsonString: String
        
        if action == "add" {
            jsonString = "{\"action\":\"\(action)\",\"peer_ip\":\"\(ip)\",\"port\":\(port),\"name\":\"\(name)\"}\n"
        } else {
            jsonString = "{\"action\":\"\(action)\",\"peer_ip\":\"\(ip)\"}\n"
        }
        
        guard let data = jsonString.data(using: .utf8) else { return }
        
        socketConnection?.send(content: data, completion: .contentProcessed({ error in
            if let error = error {
                print("Socket send error: \(error)")
            }
        }))
    }
    
    func stopScanning() {
        centralManager?.stopScan()
        cleanupTimer?.invalidate()
        socketConnection?.cancel()
        print("Scanner safely stopped.")
    }
}