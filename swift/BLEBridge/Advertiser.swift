import Foundation
import CoreBluetooth
import Darwin

final class WorkerBLEAdvertiser: NSObject, CBPeripheralManagerDelegate {
    private var peripheralManager: CBPeripheralManager!

    private let serviceUUID = CBUUID(string: "8530AD31-BC8A-4A39-82E2-787A106F0F25")
    // Must match payloadCharacteristicUUID in Scanner.swift.
    private let payloadCharacteristicUUID = CBUUID(string: "2C2A0E22-2F45-4A5C-8A0F-7C1D9A8E6B31")
    private let handshakePort = 7946
    private var payloadData = Data()
    private var payloadCharacteristic: CBMutableCharacteristic?
    private var hasRegisteredService = false

    override init() {
        super.init()
        peripheralManager = CBPeripheralManager(delegate: self, queue: nil)
    }

    func peripheralManagerDidUpdateState(_ peripheral: CBPeripheralManager) {
        if peripheral.state == .poweredOn {
            startBroadcasting()
        } else {
            print("Worker BLE unavailable. State: \(peripheral.state.rawValue)")
        }
    }

    func peripheralManager(_ peripheral: CBPeripheralManager, didStartAdvertising error: Error?) {
        if let error {
            print("Advertising failed: \(error.localizedDescription)")
        } else {
            print("Advertising confirmed started.")
        }
    }

    func peripheralManager(_ peripheral: CBPeripheralManager, didAdd service: CBService, error: Error?) {
        if let error {
            print("Failed to add BLE service: \(error.localizedDescription)")
            return
        }

        hasRegisteredService = true
        let advertisementData: [String: Any] = [
            CBAdvertisementDataServiceUUIDsKey: [serviceUUID]
        ]
        peripheralManager.startAdvertising(advertisementData)
        print("Worker service registered. Advertising service UUID only.")
    }

    func peripheralManager(_ peripheral: CBPeripheralManager, didReceiveRead request: CBATTRequest) {
        guard request.characteristic.uuid == payloadCharacteristicUUID else {
            peripheral.respond(to: request, withResult: .attributeNotFound)
            return
        }

        guard request.offset <= payloadData.count else {
            peripheral.respond(to: request, withResult: .invalidOffset)
            return
        }

        request.value = payloadData.subdata(in: request.offset..<payloadData.count)
        peripheral.respond(to: request, withResult: .success)
    }

    func stopAdvertising() {
        peripheralManager?.stopAdvertising()
        peripheralManager?.removeAllServices()
        hasRegisteredService = false
        print("Worker advertising stopped.")
    }

    private func startBroadcasting() {
        guard peripheralManager.state == .poweredOn else { return }

        let lanIP = getLocalIPv4Address() ?? "0.0.0.0"
        let hostname = (Host.current().localizedName ?? "Worker")
            .replacingOccurrences(of: "|", with: "-")
        let payloadString = "\(hostname)|\(lanIP)|\(handshakePort)"
        payloadData = Data(payloadString.utf8)

        guard !hasRegisteredService else {
            print("Worker service already registered. Payload ready: \(payloadString)")
            return
        }

        let characteristic = CBMutableCharacteristic(
            type: payloadCharacteristicUUID,
            properties: [.read],
            // value: nil keeps the characteristic dynamic so the current payload
            // is served on demand via didReceiveRead.
            value: nil,
            permissions: [.readable]
        )
        payloadCharacteristic = characteristic

        let service = CBMutableService(type: serviceUUID, primary: true)
        service.characteristics = [characteristic]

        peripheralManager.add(service)
        print("Registering worker payload characteristic: \(payloadString)")
    }

    private func getLocalIPv4Address() -> String? {
        var address: String?
        var ifaddr: UnsafeMutablePointer<ifaddrs>?

        if getifaddrs(&ifaddr) == 0 {
            var ptr = ifaddr
            while ptr != nil {
                defer { ptr = ptr?.pointee.ifa_next }

                guard let interface = ptr?.pointee,
                      let addr = interface.ifa_addr else { continue }
                let addrFamily = addr.pointee.sa_family

                if addrFamily == UInt8(AF_INET) {
                    let name = String(cString: interface.ifa_name)

                    if name.hasPrefix("en"), address == nil {
                        var hostBuffer = [CChar](repeating: 0, count: Int(NI_MAXHOST))
                        getnameinfo(addr, socklen_t(addr.pointee.sa_len),
                                    &hostBuffer, socklen_t(hostBuffer.count),
                                    nil, socklen_t(0), NI_NUMERICHOST)
                        address = String(cString: hostBuffer)
                    }
                }
            }
            freeifaddrs(ifaddr)
        }
        return address
    }
}
