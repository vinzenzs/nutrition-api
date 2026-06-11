import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:permission_handler/permission_handler.dart';

import '../../data/repository.dart';
import '../../state/app_providers.dart';
import '../../state/scan_provider.dart';
import 'photo_confirm.dart';
import 'product_card.dart';

enum CameraMode { scan, photo }

/// Camera-first input. A segmented control at the top switches between scan
/// (barcode → product card → log) and photo (capture → vision → confirm).
class CameraPage extends ConsumerStatefulWidget {
  const CameraPage({super.key});

  @override
  ConsumerState<CameraPage> createState() => _CameraPageState();
}

class _CameraPageState extends ConsumerState<CameraPage> {
  CameraMode _mode = CameraMode.scan;
  final _scanController = MobileScannerController();
  bool _busy = false;

  @override
  void dispose() {
    _scanController.dispose();
    super.dispose();
  }

  void _switchTo(CameraMode mode) {
    setState(() => _mode = mode);
    if (mode == CameraMode.scan) {
      ref.read(scanProvider.notifier).reset();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Camera'),
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(56),
          child: Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: SegmentedButton<CameraMode>(
              segments: const [
                ButtonSegment(
                    value: CameraMode.scan,
                    icon: Icon(Icons.qr_code_scanner),
                    label: Text('Scan')),
                ButtonSegment(
                    value: CameraMode.photo,
                    icon: Icon(Icons.photo_camera_outlined),
                    label: Text('Photo')),
              ],
              selected: {_mode},
              onSelectionChanged: (s) => _switchTo(s.first),
            ),
          ),
        ),
      ),
      body: _mode == CameraMode.scan ? _scanBody() : _photoBody(),
    );
  }

  // --- Scan mode ------------------------------------------------------------

  Widget _scanBody() {
    final scan = ref.watch(scanProvider);
    return Stack(
      fit: StackFit.expand,
      children: [
        MobileScanner(
          controller: _scanController,
          onDetect: (capture) {
            final raw = capture.barcodes.isNotEmpty
                ? capture.barcodes.first.rawValue
                : null;
            if (raw != null) ref.read(scanProvider.notifier).onBarcode(raw);
          },
          errorBuilder: (context, error, child) => const _PermissionDenied(
            message: 'Camera access is needed to scan barcodes.',
          ),
        ),
        if (scan.phase == ScanPhase.lookingUp)
          const Center(child: CircularProgressIndicator()),
        if (scan.phase == ScanPhase.product && scan.product != null)
          Align(
            alignment: Alignment.bottomCenter,
            child: ProductCard(
              product: scan.product!,
              quantityG: scan.quantityG,
              mealType: scan.mealType,
              onQuantityChanged:
                  ref.read(scanProvider.notifier).setQuantity,
              onMealTypeChanged:
                  ref.read(scanProvider.notifier).setMealType,
              onLog: () => _logScanned(),
              onDismiss: () => ref.read(scanProvider.notifier).reset(),
            ),
          ),
        if (scan.phase == ScanPhase.notFound)
          Align(
            alignment: Alignment.bottomCenter,
            child: _NotFoundSheet(
              barcode: scan.barcode ?? '',
              onDescribe: _describeFreeform,
              onPhoto: () => _switchTo(CameraMode.photo),
              onDismiss: () => ref.read(scanProvider.notifier).reset(),
            ),
          ),
      ],
    );
  }

  Future<void> _logScanned() async {
    await ref.read(scanProvider.notifier).log();
    if (!mounted) return;
    ScaffoldMessenger.of(context)
        .showSnackBar(const SnackBar(content: Text('Meal logged')));
    ref.read(scanProvider.notifier).reset();
  }

  // --- Photo mode -----------------------------------------------------------

  Widget _photoBody() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.photo_camera_outlined, size: 72),
          const SizedBox(height: 16),
          const Text('Snap a photo of your meal'),
          const SizedBox(height: 24),
          FilledButton.icon(
            icon: const Icon(Icons.camera_alt),
            label: Text(_busy ? 'Analyzing…' : 'Capture'),
            onPressed: _busy ? null : _capturePhoto,
          ),
        ],
      ),
    );
  }

  Future<void> _capturePhoto() async {
    // imageQuality forces a JPEG re-encode so HEIC never reaches the backend.
    final picker = ImagePicker();
    final XFile? shot = await picker.pickImage(
      source: ImageSource.camera,
      imageQuality: 85,
      maxWidth: 1568,
    );
    if (shot == null) return;
    if (!mounted) return;
    setState(() => _busy = true);
    try {
      final bytes = await shot.readAsBytes();
      final result = await ref.read(repositoryProvider).logMealFromPhoto(
            jpegBytes: bytes,
            quantityG: 100,
            mealType: mealTypeForNow(),
            loggedAt: DateTime.now(),
          );
      if (!mounted) return;
      await showPhotoConfirm(context, ref, result);
    } on VisionUnavailable {
      if (mounted) await _visionUnavailableSheet();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Photo log failed: $e')));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _visionUnavailableSheet() {
    return showModalBottomSheet<void>(
      context: context,
      builder: (sheetContext) => Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Text(
              "Photo logging isn't configured on this server. "
              'Describe the meal instead?',
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            FilledButton(
              onPressed: () {
                Navigator.of(sheetContext).pop();
                _describeFreeform();
              },
              child: const Text('Describe instead'),
            ),
          ],
        ),
      ),
    );
  }

  // --- Shared: freeform escape hatch ---------------------------------------

  Future<void> _describeFreeform() async {
    ref.read(scanProvider.notifier).reset();
    await showFreeformSheet(context, ref);
  }
}

/// Empty state shown when camera permission is denied.
class _PermissionDenied extends StatelessWidget {
  final String message;
  const _PermissionDenied({required this.message});

  @override
  Widget build(BuildContext context) {
    return Container(
      color: Theme.of(context).colorScheme.surface,
      padding: const EdgeInsets.all(32),
      alignment: Alignment.center,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.no_photography_outlined, size: 64),
          const SizedBox(height: 16),
          Text(message, textAlign: TextAlign.center),
          const SizedBox(height: 24),
          FilledButton(
            onPressed: openAppSettings,
            child: const Text('Open Settings'),
          ),
        ],
      ),
    );
  }
}

class _NotFoundSheet extends StatelessWidget {
  final String barcode;
  final VoidCallback onDescribe;
  final VoidCallback onPhoto;
  final VoidCallback onDismiss;

  const _NotFoundSheet({
    required this.barcode,
    required this.onDescribe,
    required this.onPhoto,
    required this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.all(16),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Expanded(child: Text("We don't know this barcode yet")),
                IconButton(
                    onPressed: onDismiss, icon: const Icon(Icons.close)),
              ],
            ),
            Text(barcode, style: Theme.of(context).textTheme.bodySmall),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton.icon(
                    icon: const Icon(Icons.edit_outlined),
                    label: const Text('Describe it'),
                    onPressed: onDescribe,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: FilledButton.icon(
                    icon: const Icon(Icons.photo_camera_outlined),
                    label: const Text('Take a photo'),
                    onPressed: onPhoto,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
