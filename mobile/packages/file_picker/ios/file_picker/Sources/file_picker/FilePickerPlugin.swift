import Flutter
import UIKit

public final class FilePickerPlugin: NSObject, FlutterPlugin, UIDocumentPickerDelegate {
    private var pendingResult: FlutterResult?

    public static func register(with registrar: FlutterPluginRegistrar) {
        let channel = FlutterMethodChannel(
            name: "spendable_today/file_picker",
            binaryMessenger: registrar.messenger()
        )
        let instance = FilePickerPlugin()
        registrar.addMethodCallDelegate(instance, channel: channel)
    }

    public func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
        guard call.method == "pickFiles" else {
            result(FlutterMethodNotImplemented)
            return
        }
        guard pendingResult == nil else {
            result(
                FlutterError(
                    code: "already_active",
                    message: "A document picker is already open.",
                    details: nil
                )
            )
            return
        }
        pendingResult = result
        let picker = UIDocumentPickerViewController(
            documentTypes: ["public.comma-separated-values-text", "public.plain-text"],
            in: .import
        )
        picker.delegate = self
        registrarViewController()?.present(picker, animated: true)
    }

    public func documentPicker(
        _ controller: UIDocumentPickerViewController,
        didPickDocumentsAt urls: [URL]
    ) {
        guard let url = urls.first else {
            finish(nil)
            return
        }
        let size = ((try? url.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0)
        finish([
            "name": url.lastPathComponent,
            "path": url.path,
            "size": size
        ])
    }

    public func documentPickerWasCancelled(_ controller: UIDocumentPickerViewController) {
        finish(nil)
    }

    private func finish(_ value: Any?) {
        pendingResult?(value)
        pendingResult = nil
    }

    private func registrarViewController() -> UIViewController? {
        let scenes = UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
        return scenes
            .flatMap(\.windows)
            .first(where: \.isKeyWindow)?
            .rootViewController
    }
}
