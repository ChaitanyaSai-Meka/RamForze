import Foundation
import CoreBluetooth
import Darwin

final class WorkerBLEAdvertiser: NSObject, CBPeripheralManagerDelegate {
    private var peripheralManager: CBPeripheralManager!

    private let serviceUUID = CBUUID(string: "8530AD31-BC8A-4A39-82E2-787A106F0F25")
    private let handshakePort = 7946
    private let maxLocalNameBytes = 28

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
        let hostname = (Host.current().localizedName ?? "Worker")
            .replacingOccurrences(of: "|", with: "-")
        let payloadString = makePayloadString(hostname: hostname, lanIP: lanIP)

        let advertisementData: [String: Any] = [
            CBAdvertisementDataLocalNameKey: payloadString,
            CBAdvertisementDataServiceUUIDsKey: [serviceUUID]
        ]

        peripheralManager.startAdvertising(advertisementData)
        print("Worker advertising started. Name payload: \(payloadString)")
    }

    private func makePayloadString(hostname: String, lanIP: String) -> String {
        let suffix = "|\(lanIP)|\(handshakePort)"
        var truncatedHostname = hostname

        while !truncatedHostname.isEmpty && (truncatedHostname + suffix).utf8.count > maxLocalNameBytes {
            truncatedHostname.removeLast()
        }

        return truncatedHostname + suffix
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
