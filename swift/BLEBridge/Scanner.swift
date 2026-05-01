import Foundation
import CoreBluetooth
import Network

private struct WorkerInfo {
    var ip: String
    var port: Int
    var name: String
    var lastSeen: Date
}

final class MasterBLEScanner: NSObject, CBCentralManagerDelegate, CBPeripheralDelegate {
    
    private var centralManager: CBCentralManager!
    
    private let serviceUUID = BLEConstants.serviceUUID
    private let payloadCharacteristicUUID = BLEConstants.payloadCharacteristicUUID
    
    private var activeWorkers: [String: WorkerInfo] = [:]
    private var cleanupTimer: Timer?
    private var pendingPeripherals: [String: CBPeripheral] = [:]
    private var pendingReads: Set<String> = []
    
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
        let deviceID = peripheral.identifier.uuidString
        if var knownWorker = activeWorkers[deviceID] {
            knownWorker.lastSeen = Date()
            activeWorkers[deviceID] = knownWorker
            return
        }

        guard !pendingReads.contains(deviceID) else { return }

        pendingReads.insert(deviceID)
        pendingPeripherals[deviceID] = peripheral
        peripheral.delegate = self
        centralManager.connect(peripheral, options: nil)
    }

    func centralManager(_ central: CBCentralManager, didConnect peripheral: CBPeripheral) {
        peripheral.discoverServices([serviceUUID])
    }

    func centralManager(_ central: CBCentralManager, didFailToConnect peripheral: CBPeripheral, error: Error?) {
        let deviceID = peripheral.identifier.uuidString
        pendingReads.remove(deviceID)
        pendingPeripherals.removeValue(forKey: deviceID)
        if let error {
            print("Failed to connect to worker peripheral: \(error.localizedDescription)")
        }
    }

    func centralManager(_ central: CBCentralManager, didDisconnectPeripheral peripheral: CBPeripheral, error: Error?) {
        let deviceID = peripheral.identifier.uuidString
        if pendingReads.contains(deviceID) {
            if let error {
                print("Peripheral disconnected during read: \(error.localizedDescription)")
            } else {
                print("Peripheral disconnected unexpectedly during read: \(deviceID)")
            }
        }
        pendingReads.remove(deviceID)
        pendingPeripherals.removeValue(forKey: deviceID)
    }

    func peripheral(_ peripheral: CBPeripheral, didDiscoverServices error: Error?) {
        if let error {
            print("Failed to discover services: \(error.localizedDescription)")
            cleanupPendingPeripheral(peripheral)
            return
        }

        guard let service = peripheral.services?.first(where: { $0.uuid == serviceUUID }) else {
            cleanupPendingPeripheral(peripheral)
            return
        }

        peripheral.discoverCharacteristics([payloadCharacteristicUUID], for: service)
    }

    func peripheral(_ peripheral: CBPeripheral, didDiscoverCharacteristicsFor service: CBService, error: Error?) {
        if let error {
            print("Failed to discover characteristics: \(error.localizedDescription)")
            cleanupPendingPeripheral(peripheral)
            return
        }

        guard let characteristic = service.characteristics?.first(where: { $0.uuid == payloadCharacteristicUUID }) else {
            cleanupPendingPeripheral(peripheral)
            return
        }

        peripheral.readValue(for: characteristic)
    }

    func peripheral(_ peripheral: CBPeripheral, didUpdateValueFor characteristic: CBCharacteristic, error: Error?) {
        defer { cleanupPendingPeripheral(peripheral) }

        if let error {
            print("Failed to read worker payload: \(error.localizedDescription)")
            return
        }

        guard characteristic.uuid == payloadCharacteristicUUID,
              let data = characteristic.value,
              let payload = String(data: data, encoding: .utf8) else { return }

        let parts = payload.split(separator: "|").map { String($0) }
        guard parts.count == 3,
              let port = Int(parts[2]),
              (1...65535).contains(port) else { return }

        let deviceID = peripheral.identifier.uuidString
        let hostname = parts[0]
        let ipAddress = parts[1]
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
    
    private func streamNDJSON(action: String, ip: String, port: Int = 7946, name: String = "") {
        var dict: [String: Any] = ["action": action, "peer_ip": ip]
        if action == "add" {
            dict["port"] = port
            dict["name"] = name
        }
        guard let data = try? JSONSerialization.data(withJSONObject: dict),
              var line = String(data: data, encoding: .utf8) else { return }
        line += "\n"
        socketConnection?.send(content: line.data(using: .utf8), completion: .contentProcessed({ error in
            if let error { print("Socket send error: \(error)") }
        }))
    }

    private func cleanupPendingPeripheral(_ peripheral: CBPeripheral) {
        let deviceID = peripheral.identifier.uuidString
        pendingReads.remove(deviceID)
        pendingPeripherals.removeValue(forKey: deviceID)
        centralManager.cancelPeripheralConnection(peripheral)
    }
    
    func stopScanning() {
        centralManager?.stopScan()
        cleanupTimer?.invalidate()
        for peripheral in pendingPeripherals.values {
            centralManager.cancelPeripheralConnection(peripheral)
        }
        pendingPeripherals.removeAll()
        pendingReads.removeAll()
        socketConnection?.cancel()
        print("Scanner safely stopped.")
    }
}
