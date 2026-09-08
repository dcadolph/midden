import SwiftUI

/// TagChip is a tappable tag pill used for both choosing and browsing tags.
struct TagChip: View {
    /// tag is the label without a leading hash character.
    let tag: String
    /// count, when positive, is shown after the label.
    var count: Int = 0
    /// isSelected draws the chip in the accent color.
    let isSelected: Bool
    /// action runs when the chip is tapped.
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Text(label)
                .font(.caption)
                .padding(.horizontal, 14)
                .frame(minHeight: 44)
                .background(background, in: Capsule())
                // Without an explicit shape the tap area follows the text
                // glyphs rather than the pill, which makes the chip hard to
                // hit and leaves the padding dead.
                .contentShape(Capsule())
        }
        .buttonStyle(.borderless)
    }

    /// label is the chip text, with the count appended when there is one.
    private var label: String {
        count > 0 ? "#\(tag) \(count)" : "#\(tag)"
    }

    /// background is the pill fill for the current selection state.
    private var background: Color {
        isSelected ? Color.accentColor.opacity(0.25) : Color.secondary.opacity(0.15)
    }
}
