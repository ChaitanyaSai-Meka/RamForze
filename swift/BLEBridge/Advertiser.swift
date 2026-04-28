import Foundation
import CoreBluetooth

final class WorkerBLEAdvertiser: NSObject, CBPeripheralManagerDelegate {
    private var peripheralManager: CBPeripheralManager!

    private let serviceUUID = CBUUID(string: "8530AD31-BC8A-4A39-82E2-787A106F0F25")
    private let handshakePort = 7946

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

    func stopAdvertising() {
        peripheralManager?.stopAdvertising()
        print("Worker advertising stopped.")
    }

    private func startBroadcasting() {
        guard peripheralManager.state == .poweredOn else { return }

        let lanIP = getLocalIPv4Address() ?? "0.0.0.0"
        let hostname = Host.current().localizedName ?? "Worker"

        let payloadString = "\(hostname)|\(lanIP)|\(handshakePort)"

        let advertisementData: [String: Any] = [
            CBAdvertisementDataLocalNameKey: payloadString,
            CBAdvertisementDataServiceUUIDsKey: [serviceUUID]
        ]

        peripheralManager.startAdvertising(advertisementData)
        print("Worker advertising started. Name payload: \(payloadString)")
    }

    private func getLocalIPv4Address() -> String? {
        var address: String?
        var ifaddr: UnsafeMutablePointer<ifaddrs>?

        if getifaddrs(&ifaddr) == 0 {
            var ptr = ifaddr
            while ptr != nil {
                defer { ptr = ptr?.pointee.ifa_next }

                guard let interface = ptr?.pointee else { continue }
                let addrFamily = interface.ifa_addr.pointee.sa_family

                if addrFamily == UInt8(AF_INET) {
                    let name = String(cString: interface.ifa_name)

                    if name.hasPrefix("en"), address == nil {
                        var hostname = [CChar](repeating: 0, count: Int(NI_MAXHOST))
                        getnameinfo(interface.ifa_addr, socklen_t(interface.ifa_addr.pointee.sa_len),
                                    &hostname, socklen_t(hostname.count),
                                    nil, socklen_t(0), NI_NUMERICHOST)
                        address = String(cString: hostname)
                    }
                }
            }
            freeifaddrs(ifaddr)
        }
        return address
    }
}
