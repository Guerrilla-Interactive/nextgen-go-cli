package shared

import "strings"

// WrapText wraps s to the given width breaking at spaces; it preserves
// existing newlines and does not break words unless a single word exceeds width.
func WrapText(s string, width int) string {
    if width <= 0 {
        return s
    }
    var out []string
    for _, para := range strings.Split(s, "\n") {
        words := strings.Fields(para)
        if len(words) == 0 {
            out = append(out, "")
            continue
        }
        line := words[0]
        for _, w := range words[1:] {
            if len(line)+1+len(w) <= width {
                line += " " + w
            } else {
                out = append(out, line)
                line = w
            }
        }
        out = append(out, line)
    }
    return strings.Join(out, "\n")
}

// TruncateLines limits the string to at most max lines, splitting on \n.
// If max <= 0 or the input has fewer lines, the original string is returned.
func TruncateLines(s string, max int) string {
    if max <= 0 {
        return s
    }
    lines := strings.Split(s, "\n")
    if len(lines) <= max {
        return s
    }
    return strings.Join(lines[:max], "\n")
}

