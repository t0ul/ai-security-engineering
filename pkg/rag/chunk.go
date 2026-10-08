package rag

import (
	"regexp"
	"strings"
)

// Clean normalizes ingested text before it is indexed: raw emails carry runs of
// spaces, hard-wrapped lines, and stacks of blank lines that bloat the index and
// make retrieved passages ugly to cite. Clean standardizes line endings, strips
// control/zero-width characters, collapses intra-line whitespace, trims each line,
// and caps blank runs at a single blank line. It preserves words and paragraph
// breaks, so FTS matching and chunking are unaffected — only noise is removed.
func Clean(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = stripNoise(s)
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		ln = strings.ReplaceAll(ln, "\t", " ")
		ln = reInlineWS.ReplaceAllString(ln, " ")
		lines[i] = strings.TrimSpace(ln)
	}
	s = strings.Join(lines, "\n")
	s = reBlankRuns.ReplaceAllString(s, "\n\n") // 3+ newlines -> one blank line
	return strings.TrimSpace(s)
}

// stripNoise drops control characters (except newline/tab) and zero-width/BOM runes,
// which carry no meaning but bloat the index and corrupt cited passages.
func stripNoise(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r == 0xFEFF || (r >= 0x200B && r <= 0x200D): // BOM + zero-width space/joiners
			return -1
		case r < 0x20 || r == 0x7F: // C0 control chars + DEL
			return -1
		default:
			return r
		}
	}, s)
}

var (
	reInlineWS  = regexp.MustCompile(`[ ]{2,}`)
	reBlankRuns = regexp.MustCompile(`\n{3,}`)
	reParaSplit = regexp.MustCompile(`\n{2,}`)
)

// Chunker splits a cleaned document into the units that get indexed and retrieved.
// Different strategies trade recall vs precision: whole-document keeps full context
// but returns noisy passages; paragraph/fixed return focused passages. The input is
// already Clean()ed.
type Chunker interface {
	Name() string
	Chunk(cleaned string) []string
}

// WholeChunker indexes the whole document as one unit (the original behavior). Best
// recall, least precision — a hit returns the entire email.
type WholeChunker struct{}

func (WholeChunker) Name() string { return "whole" }
func (WholeChunker) Chunk(t string) []string {
	if strings.TrimSpace(t) == "" {
		return nil
	}
	return []string{t}
}

// ParagraphChunker splits on blank lines and greedily packs paragraphs into chunks
// up to MaxChars, so a retrieved hit is a coherent passage rather than a whole email.
type ParagraphChunker struct{ MaxChars int }

func (ParagraphChunker) Name() string { return "paragraph" }
func (p ParagraphChunker) Chunk(t string) []string {
	max := p.MaxChars
	if max <= 0 {
		max = 600
	}
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}
	for _, para := range reParaSplit.Split(t, -1) {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if cur.Len() > 0 && cur.Len()+len(para)+2 > max {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(para)
		// a single paragraph bigger than max stands alone.
		if cur.Len() >= max {
			flush()
		}
	}
	flush()
	return out
}

// FixedChunker slides a fixed-size window (in runes) over the text with an overlap,
// so long passages are split predictably and context straddling a boundary is not
// lost. Classic fixed-size chunking.
type FixedChunker struct{ Size, Overlap int }

func (FixedChunker) Name() string { return "fixed" }
func (f FixedChunker) Chunk(t string) []string {
	t = strings.TrimSpace(t)
	if t == "" {
		return nil
	}
	size := f.Size
	if size <= 0 {
		size = 500
	}
	overlap := f.Overlap
	if overlap < 0 || overlap >= size {
		overlap = size / 6
	}
	runes := []rune(t)
	if len(runes) <= size {
		return []string{t}
	}
	var out []string
	step := size - overlap
	for start := 0; start < len(runes); start += step {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, strings.TrimSpace(string(runes[start:end])))
		if end == len(runes) {
			break
		}
	}
	return out
}

// ChunkerByName resolves a configured chunking strategy; an unknown or empty name
// falls back to whole-document (the safe default). This is the single place that
// maps the operator's choice to a strategy.
func ChunkerByName(name string) Chunker {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "paragraph":
		return ParagraphChunker{MaxChars: 600}
	case "fixed":
		return FixedChunker{Size: 500, Overlap: 80}
	default:
		return WholeChunker{}
	}
}

// ChunkerNames lists the selectable strategies (for the UI).
func ChunkerNames() []string { return []string{"whole", "paragraph", "fixed"} }
