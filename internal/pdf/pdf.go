// Package pdf makes Oriel's documents: a buyer's letter of offer and the
// price list. They are drawn in pure Go with the site's own fonts, so they
// look like the rest of Oriel and need nothing installed on the server.
package pdf

import (
	"embed"
	"fmt"
	"strings"
	"time"

	"github.com/signintech/gopdf"
)

//go:embed fonts/*.ttf
var fontFiles embed.FS

// Page geometry, in points (A4).
const (
	pageW   = 595.28
	pageH   = 841.89
	marginL = 56.0
	marginR = 56.0
	marginT = 52.0
	bottom  = 780.0 // content stops here; the footer sits below
	right   = pageW - marginR
	width   = right - marginL
)

type colour struct{ r, g, b uint8 }

var (
	graphite = colour{26, 29, 31}
	inkSoft  = colour{86, 92, 97}
	inkFaint = colour{129, 135, 140}
	cobalt   = colour{31, 59, 209}
	rule     = colour{214, 216, 211}
	ruleDark = colour{120, 124, 128}
	plaster  = colour{244, 245, 242}
	wash     = colour{223, 227, 247}
)

const (
	sans    = "sans"
	semi    = "semi"
	display = "display"
	mono    = "mono"
)

// doc is a document being drawn, with the running page and footer text.
type doc struct {
	gp     gopdf.GoPdf
	footer string
	pages  int
	y      float64
}

func newDoc(title, subject, footer string) (*doc, error) {
	d := &doc{footer: footer}
	d.gp.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4, Unit: gopdf.UnitPT})
	d.gp.SetInfo(gopdf.PdfInfo{Title: title, Subject: subject, Author: "Oriel Residences Limited", Creator: "Oriel", CreationDate: time.Now()})
	for name, file := range map[string]string{
		sans: "MonaSans-Regular.ttf", semi: "MonaSans-SemiBold.ttf", display: "MonaSans-Display.ttf", mono: "MartianMono-Regular.ttf",
	} {
		data, err := fontFiles.ReadFile("fonts/" + file)
		if err != nil {
			return nil, err
		}
		if err := d.gp.AddTTFFontData(name, data); err != nil {
			return nil, fmt.Errorf("font %s: %w", file, err)
		}
	}
	d.newPage()
	return d, nil
}

func (d *doc) newPage() {
	if d.pages > 0 {
		d.drawFooter()
	}
	d.gp.AddPage()
	d.pages++
	d.y = marginT
}

// ensure starts a new page if h points will not fit.
func (d *doc) ensure(h float64) {
	if d.y+h > bottom {
		d.newPage()
	}
}

func (d *doc) font(name string, size float64, c colour) {
	d.gp.SetFont(name, "", size)
	d.gp.SetTextColor(c.r, c.g, c.b)
}

// text draws s with its baseline-ish top at (x, y).
func (d *doc) text(x, y float64, name string, size float64, c colour, s string) {
	d.font(name, size, c)
	d.gp.SetXY(x, y)
	d.gp.Text(s)
}

// spaced draws letter-spaced capitals, as the site's labels are.
func (d *doc) spaced(x, y float64, name string, size, spacing float64, c colour, s string) {
	d.font(name, size, c)
	d.gp.SetCharSpacing(spacing)
	d.gp.SetXY(x, y)
	d.gp.Text(s)
	d.gp.SetCharSpacing(0)
}

func (d *doc) measure(name string, size float64, s string) float64 {
	d.gp.SetFont(name, "", size)
	w, _ := d.gp.MeasureTextWidth(s)
	return w
}

// rightText draws s ending at x.
func (d *doc) rightText(x, y float64, name string, size float64, c colour, s string) {
	d.text(x-d.measure(name, size, s), y, name, size, c, s)
}

// para draws wrapped text from the running y and moves y below it.
func (d *doc) para(x, w float64, name string, size, leading float64, c colour, s string) {
	d.font(name, size, c)
	lines, err := d.gp.SplitTextWithWordWrap(s, w)
	if err != nil {
		lines = []string{s}
	}
	for _, line := range lines {
		d.ensure(leading)
		d.gp.SetXY(x, d.y)
		d.gp.Text(line)
		d.y += leading
	}
}

func (d *doc) line(x1, y1, x2, y2, w float64, c colour) {
	d.gp.SetLineWidth(w)
	d.gp.SetStrokeColor(c.r, c.g, c.b)
	d.gp.SetLineType("solid")
	d.gp.Line(x1, y1, x2, y2)
}

func (d *doc) dashed(x1, y1, x2, y2, w float64, c colour) {
	d.gp.SetLineWidth(w)
	d.gp.SetStrokeColor(c.r, c.g, c.b)
	d.gp.SetLineType("dashed")
	d.gp.Line(x1, y1, x2, y2)
	d.gp.SetLineType("solid")
}

func (d *doc) rect(x, y, w, h float64, fill *colour, stroke *colour, lw float64) {
	style := ""
	if fill != nil {
		d.gp.SetFillColor(fill.r, fill.g, fill.b)
		style += "F"
	}
	if stroke != nil {
		d.gp.SetLineWidth(lw)
		d.gp.SetStrokeColor(stroke.r, stroke.g, stroke.b)
		style = "D" + style
	}
	if style == "DF" {
		style = "FD"
	}
	d.gp.RectFromUpperLeftWithStyle(x, y, w, h, style)
}

// header draws the wordmark and the title block every document opens with.
func (d *doc) header(title string, rows [][2]string) {
	d.spaced(marginL, marginT+2, display, 21, 7, graphite, "ORIEL")
	d.spaced(marginL, marginT+30, mono, 6.5, 1.6, inkSoft, "RIVERSIDE · NAIROBI")
	// Title block, as on a drawing sheet.
	bw := 196.0
	bx := right - bw
	rowH := 17.0
	h := 26 + rowH*float64(len(rows))
	d.rect(bx, marginT-6, bw, h, nil, &graphite, 0.8)
	d.spaced(bx+10, marginT+2, mono, 6.5, 1.4, inkSoft, strings.ToUpper(title))
	// Text is placed by its baseline, so each row's rule sits above its words.
	for i, r := range rows {
		y := marginT + 22 + rowH*float64(i)
		d.line(bx, y-11, right, y-11, 0.4, rule)
		d.text(bx+10, y, sans, 7.5, inkSoft, r[0])
		d.rightText(right-10, y, mono, 8, graphite, r[1])
	}
	d.line(marginL, marginT+h+8, right, marginT+h+8, 0.8, graphite)
	d.y = marginT + h + 34
}

func (d *doc) drawFooter() {
	d.line(marginL, pageH-46, right, pageH-46, 0.4, ruleDark)
	d.text(marginL, pageH-38, sans, 7, inkSoft, d.footer)
	d.rightText(right, pageH-38, mono, 7, inkSoft, fmt.Sprintf("Page %d", d.pages))
}

func (d *doc) bytes() ([]byte, error) {
	d.drawFooter()
	return d.gp.GetBytesPdfReturnErr()
}

// label draws a small spaced label at the running y.
func (d *doc) label(x float64, s string) {
	d.spaced(x, d.y, mono, 6.5, 1.3, inkSoft, strings.ToUpper(s))
}

func kes(n int64) string {
	s := fmt.Sprint(n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return "KES " + b.String()
}

func percent(bp int64) string {
	whole, frac := bp/100, bp%100
	switch {
	case frac == 0:
		return fmt.Sprintf("%d%%", whole)
	case frac%10 == 0:
		return fmt.Sprintf("%d.%d%%", whole, frac/10)
	}
	return fmt.Sprintf("%d.%02d%%", whole, frac)
}
