import AVFoundation
import Foundation
import Speech

/// Dictation turns speech into text on the device.
///
/// On-device recognition is required rather than preferred, so a spoken entry
/// never leaves the phone. A device or locale without an installed model
/// surfaces an error instead of silently falling back to Apple's servers.
@MainActor
final class Dictation: ObservableObject {
    /// transcript is the text recognized so far in the current session.
    @Published private(set) var transcript = ""
    /// isRecording reports whether the microphone is currently open.
    @Published private(set) var isRecording = false
    /// errorMessage carries the most recent failure for display.
    @Published var errorMessage: String?

    /// recognizer performs the speech recognition.
    private let recognizer = SFSpeechRecognizer(locale: Locale(identifier: "en-US"))
    /// engine captures microphone audio.
    private let engine = AVAudioEngine()
    /// request feeds captured audio to the recognizer.
    private var request: SFSpeechAudioBufferRecognitionRequest?
    /// task is the in-flight recognition.
    private var task: SFSpeechRecognitionTask?

    /// start requests permission if needed and begins recognizing speech.
    func start() {
        guard !isRecording else { return }
        transcript = ""
        errorMessage = nil
        SFSpeechRecognizer.requestAuthorization { [weak self] status in
            Task { @MainActor in
                guard let self else { return }
                guard status == .authorized else {
                    self.errorMessage = "Speech recognition permission was declined."
                    return
                }
                self.beginSession()
            }
        }
    }

    /// stop ends the recording and leaves the transcript in place.
    func stop() {
        guard isRecording else { return }
        engine.stop()
        engine.inputNode.removeTap(onBus: 0)
        request?.endAudio()
        task?.finish()
        request = nil
        task = nil
        isRecording = false
    }

    /// reset clears the transcript for a fresh entry.
    func reset() {
        transcript = ""
    }

    /// beginSession configures the audio session and starts the recognition task.
    private func beginSession() {
        guard let recognizer, recognizer.isAvailable else {
            errorMessage = "Speech recognition is unavailable on this device."
            return
        }
        guard recognizer.supportsOnDeviceRecognition else {
            errorMessage = "On-device speech recognition is not installed for this language."
            return
        }
        let session = AVAudioSession.sharedInstance()
        do {
            try session.setCategory(.record, mode: .measurement, options: .duckOthers)
            try session.setActive(true, options: .notifyOthersOnDeactivation)
        } catch {
            errorMessage = "Could not start the microphone: \(error.localizedDescription)"
            return
        }
        let request = SFSpeechAudioBufferRecognitionRequest()
        request.shouldReportPartialResults = true
        request.requiresOnDeviceRecognition = true
        self.request = request

        task = recognizer.recognitionTask(with: request) { [weak self] result, error in
            Task { @MainActor in
                guard let self else { return }
                if let result {
                    self.transcript = result.bestTranscription.formattedString
                }
                if error != nil || result?.isFinal == true {
                    self.stop()
                }
            }
        }

        let input = engine.inputNode
        let format = input.outputFormat(forBus: 0)
        input.installTap(onBus: 0, bufferSize: 1024, format: format) { buffer, _ in
            request.append(buffer)
        }
        engine.prepare()
        do {
            try engine.start()
            isRecording = true
        } catch {
            errorMessage = "Could not start the microphone: \(error.localizedDescription)"
            stop()
        }
    }
}
