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
          message: "A file picker is already open.",
          details: nil
        )
      )
      return
    }
    pendingResult = result
    let picker = UIDocumentPickerViewController(
      documentTypes: [
        "public.comma-separated-values-text",
        "public.plain-text",
        "public.data",
      ],
      in: .import
    )
    picker.delegate = self
    picker.allowsMultipleSelection = false
    guard let controller = topViewController() else {
      pendingResult = nil
      result(
        FlutterError(
          code: "no_view_controller",
          message: "Unable to present the file picker.",
          details: nil
        )
      )
      return
    }
    controller.present(picker, animated: true)
  }

  public func documentPicker(
    _ controller: UIDocumentPickerViewController,
    didPickDocumentsAt urls: [URL]
  ) {
    guard let url = urls.first else {
      finish(nil)
      return
    }
    let values = try? url.resourceValues(forKeys: [.fileSizeKey])
    finish([
      "name": url.lastPathComponent,
      "path": url.path,
      "size": values?.fileSize ?? 0,
    ])
  }

  public func documentPickerWasCancelled(_ controller: UIDocumentPickerViewController) {
    finish(nil)
  }

  private func finish(_ value: Any?) {
    pendingResult?(value)
    pendingResult = nil
  }

  private func topViewController(
    from root: UIViewController? = UIApplication.shared.connectedScenes
      .compactMap { $0 as? UIWindowScene }
      .flatMap(\.windows)
      .first(where: \.isKeyWindow)?
      .rootViewController
  ) -> UIViewController? {
    if let navigation = root as? UINavigationController {
      return topViewController(from: navigation.visibleViewController)
    }
    if let tab = root as? UITabBarController {
      return topViewController(from: tab.selectedViewController)
    }
    if let presented = root?.presentedViewController {
      return topViewController(from: presented)
    }
    return root
  }
}
