package org.golang.app;

import android.app.Activity;
import android.content.ContentValues;
import android.content.pm.PackageManager;
import android.graphics.ImageFormat;
import android.graphics.SurfaceTexture;
import android.hardware.camera2.CameraCaptureSession;
import android.hardware.camera2.CameraCharacteristics;
import android.hardware.camera2.CameraDevice;
import android.hardware.camera2.CameraManager;
import android.hardware.camera2.CaptureRequest;
import android.hardware.camera2.params.StreamConfigurationMap;
import android.media.Image;
import android.media.ImageReader;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.os.Handler;
import android.os.HandlerThread;
import android.provider.MediaStore;
import android.util.Log;
import android.util.Size;
import android.view.Surface;
import android.view.TextureView;

import java.io.File;
import java.io.OutputStream;
import java.nio.ByteBuffer;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * A persistent Camera2 controller. The camera is opened once and kept open so
 * the user can take multiple pictures without the camera being released between
 * shots. A live preview is rendered to a TextureView supplied by the activity.
 *
 * Each {@link #takePicture()} captures a JPEG, saves it to the media store and
 * reports the resulting content:// URI through the {@link Callback}.
 */
public class CameraController {

    public interface Callback {
        void onPhoto(String uri);
    }

    private final Activity activity;
    private final Callback callback;
    private final TextureView textureView;

    private CameraDevice cameraDevice;
    private CameraCaptureSession captureSession;
    private ImageReader imageReader;
    private HandlerThread backgroundThread;
    private Handler backgroundHandler;
    private String cameraId;
    private Size previewSize;
    private Surface previewSurface;

    private final TextureView.SurfaceTextureListener surfaceListener = new TextureView.SurfaceTextureListener() {
        @Override
        public void onSurfaceTextureAvailable(SurfaceTexture surface, int width, int height) {
            openCamera();
        }

        @Override
        public void onSurfaceTextureSizeChanged(SurfaceTexture surface, int width, int height) {
        }

        @Override
        public boolean onSurfaceTextureDestroyed(SurfaceTexture surface) {
            return true;
        }

        @Override
        public void onSurfaceTextureUpdated(SurfaceTexture surface) {
        }
    };

    public CameraController(Activity activity, TextureView textureView, Callback callback) {
        this.activity = activity;
        this.textureView = textureView;
        this.callback = callback;
    }

    /** Opens the camera session; the preview appears once the TextureView surface is ready. */
    public void open() {
        startBackgroundThread();
        if (textureView.isAvailable()) {
            openCamera();
        } else {
            textureView.setSurfaceTextureListener(surfaceListener);
        }
    }

    /** Releases the camera and background thread. Safe to call multiple times. */
    public void close() {
        if (captureSession != null) {
            captureSession.close();
            captureSession = null;
        }
        if (cameraDevice != null) {
            cameraDevice.close();
            cameraDevice = null;
        }
        if (imageReader != null) {
            imageReader.close();
            imageReader = null;
        }
        textureView.setSurfaceTextureListener(null);
        stopBackgroundThread();
    }

    /** Captures a single photo, saves it to the media store and calls back with its URI. */
    public void takePicture() {
        if (cameraDevice == null || captureSession == null || imageReader == null) {
            Log.e("Fyne", "takePicture called before the camera is ready");
            return;
        }
        try {
            final CaptureRequest.Builder builder = cameraDevice.createCaptureRequest(CameraDevice.TEMPLATE_STILL_CAPTURE);
            builder.addTarget(imageReader.getSurface());
            captureSession.capture(builder.build(), null, backgroundHandler);
        } catch (Exception e) {
            Log.e("Fyne", "takePicture failed", e);
        }
    }

    private void startBackgroundThread() {
        backgroundThread = new HandlerThread("FyneCamera");
        backgroundThread.start();
        backgroundHandler = new Handler(backgroundThread.getLooper());
    }

    private void stopBackgroundThread() {
        if (backgroundThread == null) {
            return;
        }
        backgroundThread.quitSafely();
        try {
            backgroundThread.join();
        } catch (InterruptedException e) {
            Log.e("Fyne", "thread interrupted", e);
        }
        backgroundThread = null;
        backgroundHandler = null;
    }

    private void openCamera() {
        if (activity.checkSelfPermission("android.permission.CAMERA") != PackageManager.PERMISSION_GRANTED) {
            Log.e("Fyne", "CAMERA permission not granted");
            callback.onPhoto("");
            return;
        }
        try {
            CameraManager manager = (CameraManager) activity.getSystemService(Activity.CAMERA_SERVICE);
            cameraId = selectCamera(manager);
            CameraCharacteristics characteristics = manager.getCameraCharacteristics(cameraId);
            StreamConfigurationMap map = characteristics.get(CameraCharacteristics.SCALER_STREAM_CONFIGURATION_MAP);
            previewSize = chooseSize(map.getOutputSizes(SurfaceTexture.class));

            imageReader = ImageReader.newInstance(previewSize.getWidth(), previewSize.getHeight(),
                    ImageFormat.JPEG, 2);
            imageReader.setOnImageAvailableListener(imageListener, backgroundHandler);

            SurfaceTexture texture = textureView.getSurfaceTexture();
            texture.setDefaultBufferSize(previewSize.getWidth(), previewSize.getHeight());
            previewSurface = new Surface(texture);

            manager.openCamera(cameraId, stateCallback, backgroundHandler);
        } catch (Exception e) {
            Log.e("Fyne", "openCamera failed", e);
            callback.onPhoto("");
        }
    }

    private final CameraDevice.StateCallback stateCallback = new CameraDevice.StateCallback() {
        @Override
        public void onOpened(CameraDevice device) {
            cameraDevice = device;
            createPreviewSession();
        }

        @Override
        public void onDisconnected(CameraDevice device) {
            device.close();
            cameraDevice = null;
        }

        @Override
        public void onError(CameraDevice device, int error) {
            device.close();
            cameraDevice = null;
            callback.onPhoto("");
        }
    };

    private void createPreviewSession() {
        try {
            CaptureRequest.Builder builder = cameraDevice.createCaptureRequest(CameraDevice.TEMPLATE_PREVIEW);
            builder.addTarget(previewSurface);
            List<Surface> targets = new ArrayList<>();
            targets.add(previewSurface);
            targets.add(imageReader.getSurface());

            cameraDevice.createCaptureSession(targets, new CameraCaptureSession.StateCallback() {
                @Override
                public void onConfigured(CameraCaptureSession session) {
                    captureSession = session;
                    try {
                        session.setRepeatingRequest(builder.build(), null, backgroundHandler);
                    } catch (Exception e) {
                        Log.e("Fyne", "setRepeatingRequest failed", e);
                    }
                }

                @Override
                public void onConfigureFailed(CameraCaptureSession session) {
                    Log.e("Fyne", "preview configuration failed");
                }
            }, backgroundHandler);
        } catch (Exception e) {
            Log.e("Fyne", "createPreviewSession failed", e);
        }
    }

    private final ImageReader.OnImageAvailableListener imageListener = new ImageReader.OnImageAvailableListener() {
        @Override
        public void onImageAvailable(ImageReader reader) {
            Image image = reader.acquireLatestImage();
            if (image != null) {
                handleImage(image);
            }
        }
    };

    private void handleImage(Image image) {
        final byte[] bytes;
        try {
            ByteBuffer buffer = image.getPlanes()[0].getBuffer();
            bytes = new byte[buffer.remaining()];
            buffer.get(bytes);
        } finally {
            image.close();
        }
        final String uri = saveImage(bytes);
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                callback.onPhoto(uri);
            }
        });
    }

    private String saveImage(byte[] jpeg) {
        try {
            String name = "IMG_" + System.currentTimeMillis() + ".jpg";
            ContentValues values = new ContentValues();
            values.put(MediaStore.Images.Media.DISPLAY_NAME, name);
            values.put(MediaStore.Images.Media.MIME_TYPE, "image/jpeg");
            Uri uri;
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                values.put(MediaStore.Images.Media.RELATIVE_PATH, Environment.DIRECTORY_PICTURES);
                uri = activity.getContentResolver().insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, values);
            } else {
                File dir = Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_PICTURES);
                if (!dir.exists()) {
                    dir.mkdirs();
                }
                File file = new File(dir, name);
                uri = Uri.fromFile(file);
            }
            if (uri == null) {
                return "";
            }
            OutputStream out = activity.getContentResolver().openOutputStream(uri);
            if (out == null) {
                return "";
            }
            out.write(jpeg);
            out.close();
            return uri.toString();
        } catch (Exception e) {
            Log.e("Fyne", "saveImage failed", e);
            return "";
        }
    }

    private String selectCamera(CameraManager manager) throws Exception {
        for (String id : manager.getCameraIdList()) {
            CameraCharacteristics c = manager.getCameraCharacteristics(id);
            Integer facing = c.get(CameraCharacteristics.LENS_FACING);
            if (facing != null && facing == CameraCharacteristics.LENS_FACING_BACK) {
                return id;
            }
        }
        return manager.getCameraIdList()[0];
    }

    private Size chooseSize(Size[] sizes) {
        if (sizes == null || sizes.length == 0) {
            return new Size(1280, 720);
        }
        Size best = sizes[0];
        for (Size s : sizes) {
            if (Math.abs(s.getHeight() - 720) < Math.abs(best.getHeight() - 720)) {
                best = s;
            } else if (s.getHeight() == best.getHeight() && s.getWidth() > best.getWidth()) {
                best = s;
            }
        }
        return best;
    }
}
