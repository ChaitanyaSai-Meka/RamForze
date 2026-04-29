import Foundation
import CoreBluetooth
import Network

private struct WorkerInfo {
    var ip: String
    var port: String
    var name: String
    var lastSeen: Date
}

final class MasterBLEScanner: NSObject, CBCentralManagerDelegate {
    
    private var centralManager: CBCentralManager!
    
    private let serviceUUID = CBUUID(string: "8530AD31-BC8A-4A39-82E2-787A106F0F25")
    
    private var activeWorkers: [String: WorkerInfo] = [:]
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
                print("Scanner connected to Go backend via socket.")
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
            let deviceID = peripheral.identifier.uuidString
            let hostname = parts[0]
            let ipAddress = parts[1]
            let port = parts[2]
            let lastSeen = Date()
            
            if activeWorkers[deviceID] == nil {
                print("➕ NEW WORKER FOUND: \(hostname) (\(ipAddress):\(port))")
                streamNDJSON(action: "add", ip: ipAddress, port: port, name: hostname)
            } else if activeWorkers[deviceID]?.ip != ipAddress ||
                        activeWorkers[deviceID]?.port != port ||
                        activeWorkers[deviceID]?.name != hostname {
                print("WORKER UPDATED: \(hostname) (\(ipAddress):\(port))")
                // Re-send "add" as an upsert so the Go backend refreshes the peer entry.
                streamNDJSON(action: "add", ip: ipAddress, port: port, name: hostname)
            }
            
            activeWorkers[deviceID] = WorkerInfo(
                ip: ipAddress,
                port: port,
                name: hostname,
                lastSeen: lastSeen
            )
        }
    }
    
    private func sweepDeadWorkers() {
        let now = Date()
        var staleWorkerIDs: [String] = []
        
        for (deviceID, info) in activeWorkers {
            if now.timeIntervalSince(info.lastSeen) > 15.0 {
                print("➖ WORKER LOST: \(info.name) (\(info.ip):\(info.port)) (Timeout)")
                staleWorkerIDs.append(deviceID)
            }
        }

        for deviceID in staleWorkerIDs {
            guard let info = activeWorkers[deviceID] else { continue }
            activeWorkers.removeValue(forKey: deviceID)
            streamNDJSON(action: "remove", ip: info.ip)
        }
    }
    
    private func streamNDJSON(action: String, ip: String, port: String = "", name: String = "") {
        var dict: [String: Any] = ["action": action, "peer_ip": ip]
        if action == "add" {
            dict["port"] = Int(port) ?? 7946
            dict["name"] = name
        }
        guard let data = try? JSONSerialization.data(withJSONObject: dict),
              var line = String(data: data, encoding: .utf8) else { return }
        line += "\n"
        socketConnection?.send(content: line.data(using: .utf8), completion: .contentProcessed({ error in
            if let error { print("Socket send error: \(error)") }
        }))
    }
    
    func stopScanning() {
        centralManager?.stopScan()
        cleanupTimer?.invalidate()
        socketConnection?.cancel()
        print("Scanner safely stopped.")
    }
}
