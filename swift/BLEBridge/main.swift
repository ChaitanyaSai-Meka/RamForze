import Foundation
import Dispatch
import Darwin

print("Starting Ramforze BLE Bridge...")


let args = CommandLine.arguments

var advertiser: WorkerBLEAdvertiser?
var scanner: MasterBLEScanner?

if args.contains("--worker") {
    advertiser = WorkerBLEAdvertiser()
} else if args.contains("--master") {
    scanner = MasterBLEScanner()
} else {
    print("Usage: BLEBridge --worker | --master")
    exit(1)
}

signal(SIGTERM, SIG_IGN)
signal(SIGINT, SIG_IGN)

let sigTermSource = DispatchSource.makeSignalSource(signal: SIGTERM, queue: .main)
let sigIntSource = DispatchSource.makeSignalSource(signal: SIGINT, queue: .main)

let cleanupHandler: () -> Void = {
    print("\nBLE Bridge shutting down...")
    advertiser?.stopAdvertising()
    scanner?.stopScanning()
    exit(0)
}

sigTermSource.setEventHandler(handler: cleanupHandler)
sigIntSource.setEventHandler(handler: cleanupHandler)

sigTermSource.resume()
sigIntSource.resume()

RunLoop.main.run()
