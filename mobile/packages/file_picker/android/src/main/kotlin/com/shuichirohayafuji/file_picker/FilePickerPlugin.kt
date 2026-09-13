package com.shuichirohayafuji.file_picker

import android.app.Activity
import android.content.Intent
import android.net.Uri
import android.provider.OpenableColumns
import java.io.File
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.embedding.engine.plugins.activity.ActivityAware
import io.flutter.embedding.engine.plugins.activity.ActivityPluginBinding
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import io.flutter.plugin.common.PluginRegistry

class FilePickerPlugin : FlutterPlugin, MethodChannel.MethodCallHandler,
    ActivityAware, PluginRegistry.ActivityResultListener {
    private lateinit var channel: MethodChannel
    private var activity: Activity? = null
    private var pendingResult: MethodChannel.Result? = null

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        channel = MethodChannel(binding.binaryMessenger, "spendable_today/file_picker")
        channel.setMethodCallHandler(this)
    }

    override fun onMethodCall(call: MethodCall, result: MethodChannel.Result) {
        if (call.method != "pickFiles") {
            result.notImplemented()
            return
        }
        val currentActivity = activity
        if (currentActivity == null) {
            result.error("no_activity", "No Android activity is attached.", null)
            return
        }
        pendingResult = result
        val intent = Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
            addCategory(Intent.CATEGORY_OPENABLE)
            type = "text/*"
        }
        currentActivity.startActivityForResult(intent, requestCode)
    }

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?): Boolean {
        if (requestCode != Companion.requestCode) return false
        val result = pendingResult
        pendingResult = null
        if (resultCode != Activity.RESULT_OK || data?.data == null) {
            result?.success(null)
            return true
        }
        val uri = data.data!!
        val metadata = metadata(uri)
        val localPath = copyToCache(uri, metadata.first)
        if (localPath == null) {
            result?.error("copy_failed", "The selected CSV could not be read.", null)
            return true
        }
        result?.success(
            mapOf(
                "name" to metadata.first,
                "path" to localPath,
                "size" to metadata.second
            )
        )
        return true
    }

    private fun copyToCache(uri: Uri, originalName: String): String? {
        val currentActivity = activity ?: return null
        val safeName = originalName.replace(Regex("[^A-Za-z0-9._-]"), "_")
        val target = File(currentActivity.cacheDir, "spendable_today_${System.currentTimeMillis()}_$safeName")
        return try {
            currentActivity.contentResolver.openInputStream(uri)?.use { input ->
                target.outputStream().use { output -> input.copyTo(output) }
            } ?: return null
            target.absolutePath
        } catch (_: Exception) {
            null
        }
    }

    private fun metadata(uri: Uri): Pair<String, Long> {
        val cursor = activity?.contentResolver?.query(uri, null, null, null, null)
        cursor?.use {
            if (it.moveToFirst()) {
                val nameIndex = it.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                val sizeIndex = it.getColumnIndex(OpenableColumns.SIZE)
                val name = if (nameIndex >= 0) it.getString(nameIndex) else "import.csv"
                val size = if (sizeIndex >= 0 && !it.isNull(sizeIndex)) it.getLong(sizeIndex) else 0L
                return Pair(name, size)
            }
        }
        return Pair("import.csv", 0L)
    }

    override fun onAttachedToActivity(binding: ActivityPluginBinding) {
        activity = binding.activity
        binding.addActivityResultListener(this)
    }

    override fun onDetachedFromActivityForConfigChanges() {
        activity = null
    }

    override fun onReattachedToActivityForConfigChanges(binding: ActivityPluginBinding) {
        onAttachedToActivity(binding)
    }

    override fun onDetachedFromActivity() {
        activity = null
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        channel.setMethodCallHandler(null)
    }

    companion object {
        private const val requestCode = 8431
    }
}
