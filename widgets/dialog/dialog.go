// Copyright 2019 Graham Clark. All rights reserved.  Use of this source
// code is governed by the MIT license that can be found in the LICENSE
// file.

// Package dialog provides a modal dialog widget with support for ok/cancel.
package dialog

import (
	"fmt"

	"github.com/gcla/gowid"
	"github.com/gcla/gowid/widgets/button"
	"github.com/gcla/gowid/widgets/cellmod"
	"github.com/gcla/gowid/widgets/columns"
	"github.com/gcla/gowid/widgets/divider"
	"github.com/gcla/gowid/widgets/framed"
	"github.com/gcla/gowid/widgets/hpadding"
	"github.com/gcla/gowid/widgets/overlay"
	"github.com/gcla/gowid/widgets/pile"
	"github.com/gcla/gowid/widgets/shadow"
	"github.com/gcla/gowid/widgets/styled"
	"github.com/gcla/gowid/widgets/text"
	"github.com/gcla/tcell"
)

//======================================================================

type IWidget interface {
	gowid.IWidget
	gowid.ISettableComposite // Not ICompositeWidget - no SubWidgetSize
	GetNoFunction() gowid.IWidgetChangedCallback
	EscapeCloses() bool
	IsOpen() bool
	SetOpen(open bool, app gowid.IApp)
	SavedSubWidget() gowid.IWidget
	SetSavedSubWidget(w gowid.IWidget, app gowid.IApp)
	SavedContainer() gowid.ISettableComposite
	SetSavedContainer(c gowid.ISettableComposite, app gowid.IApp)
	Width() gowid.IWidgetDimension
	SetWidth(gowid.IWidgetDimension, gowid.IApp)
	Height() gowid.IWidgetDimension
	SetHeight(gowid.IWidgetDimension, gowid.IApp)
}

// Widget - represents a modal dialog. The bottom widget is rendered
// without the focus at full size. The bottom widget is rendered between a
// horizontal and vertical padding widget set up with the sizes provided.
//
type Widget struct {
	gowid.IWidget
	Options              Options
	savedSubWidgetWidget gowid.IWidget
	savedContainer       gowid.ISettableComposite
	contentWrapper       *gowid.ContainerWidget
	open                 bool
	NoFunction           gowid.IWidgetChangedCallback
	Callbacks            *gowid.Callbacks
}

var _ gowid.IWidget = (*Widget)(nil)
var _ IWidget = (*Widget)(nil)

type Options struct {
	Buttons         []Button
	NoShadow        bool
	NoEscapeClose   bool
	ButtonStyle     gowid.ICellStyler
	BackgroundStyle gowid.ICellStyler
}

type Button struct {
	Msg    string
	Action gowid.WidgetChangedFunction
}

var Quit, Exit, CloseD, Cancel Button
var OkCancel, ExitCancel, CloseOnly, NoButtons []Button

func init() {
	Quit = Button{
		Msg:    "Quit",
		Action: gowid.QuitFn,
	}
	Exit = Button{
		Msg:    "Exit",
		Action: gowid.QuitFn,
	}
	CloseD = Button{
		Msg: "Close",
	}
	Cancel = Button{
		Msg: "Cancel",
	}
	OkCancel = []Button{
		Button{
			Msg:    "Ok",
			Action: gowid.QuitFn,
		},
		Cancel,
	}
	ExitCancel = []Button{Exit, Cancel}
	CloseOnly = []Button{CloseD}
}

type SolidFunction func(gowid.Cell, gowid.Selector) gowid.Cell

func (f SolidFunction) Transform(c gowid.Cell, focus gowid.Selector) gowid.Cell {
	return f(c, focus)
}

// For callback registration
type OpenCloseCB struct{}
type SavedSubWidget struct{}
type SavedContainer struct{}

var (
	DefaultBackground = gowid.NewUrwidColor("white")
	DefaultButton     = gowid.NewUrwidColor("dark blue")
	DefaultButtonText = gowid.NewUrwidColor("yellow")
	DefaultText       = gowid.NewUrwidColor("black")
)

func New(content gowid.IWidget, opts ...Options) *Widget {
	var res *Widget

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	buttonStyle, backgroundStyle, borderStyle := opt.ButtonStyle, opt.BackgroundStyle, opt.BackgroundStyle

	if buttonStyle == nil {
		buttonStyle = gowid.MakePaletteEntry(DefaultButtonText, DefaultButton)
	}
	if backgroundStyle == nil {
		backgroundStyle = gowid.MakePaletteEntry(DefaultText, DefaultBackground)
	}
	if borderStyle == nil {
		borderStyle = gowid.MakePaletteEntry(DefaultButton, DefaultBackground)
	}

	colsW := make([]gowid.IContainerWidget, 0)

	pileW := make([]interface{}, 0)
	wrapper := &gowid.ContainerWidget{content, gowid.RenderWithWeight{W: 1}}
	pileW = append(pileW, wrapper)

	if len(opts) > 0 {
		for i, b := range opts[0].Buttons {
			bw := button.New(text.New(b.Msg))
			if b.Action == nil {
				bw.OnClick(gowid.WidgetCallback{fmt.Sprintf("cb-%d", i),
					func(app gowid.IApp, widget gowid.IWidget) {
						res.Close(app)
					}})
			} else {
				bw.OnClick(gowid.WidgetCallback{fmt.Sprintf("cb-%d", i), b.Action})
			}
			colsW = append(colsW,
				&gowid.ContainerWidget{
					hpadding.New(
						styled.NewExt(bw, backgroundStyle, buttonStyle),
						gowid.HAlignMiddle{},
						gowid.RenderFixed{},
					),
					gowid.RenderWithWeight{W: 1},
				},
			)
		}
	}

	if len(colsW) > 0 {
		cols := columns.New(colsW)
		pileW = append(pileW,
			styled.New(
				divider.NewUnicodeAlt(),
				borderStyle,
			),
			cols,
		)
	}

	dialogContent := pile.NewFlow(pileW...)

	frameOpts := framed.Options{
		Frame: framed.UnicodeAltFrame,
		Style: borderStyle,
	}

	var d gowid.IWidget = cellmod.Opaque(
		styled.New(
			framed.New(dialogContent, frameOpts),
			backgroundStyle,
		),
	)

	if !opt.NoShadow {
		d = shadow.New(d, 1)
	}

	res = &Widget{
		IWidget:        d,
		contentWrapper: wrapper,
		Options:        opt,
		Callbacks:      gowid.NewCallbacks(),
	}

	return res
}

func (w *Widget) String() string {
	return fmt.Sprintf("dialog")
}

func (w *Widget) SubWidget() gowid.IWidget {
	return w.IWidget
}

func (w *Widget) SetSubWidget(inner gowid.IWidget, app gowid.IApp) {
	w.IWidget = inner
}

func (w *Widget) GetNoFunction() gowid.IWidgetChangedCallback {
	return gowid.WidgetCallback{"no",
		func(app gowid.IApp, widget gowid.IWidget) {
			w.Close(app)
		}}
}

func (w *Widget) EscapeCloses() bool {
	return !w.Options.NoEscapeClose
}

func (w *Widget) IsOpen() bool {
	return w.open
}

func (w *Widget) SetOpen(open bool, app gowid.IApp) {
	prev := w.open
	w.open = open
	if prev != w.open {
		gowid.RunWidgetCallbacks(w.Callbacks, OpenCloseCB{}, app, w)
	}
}

func (w *Widget) SavedSubWidget() gowid.IWidget {
	return w.savedSubWidgetWidget
}

func (w *Widget) SetSavedSubWidget(w2 gowid.IWidget, app gowid.IApp) {
	w.savedSubWidgetWidget = w2
	gowid.RunWidgetCallbacks(w.Callbacks, SavedSubWidget{}, app, w)
}

func (w *Widget) SavedContainer() gowid.ISettableComposite {
	return w.savedContainer
}

func (w *Widget) SetSavedContainer(c gowid.ISettableComposite, app gowid.IApp) {
	w.savedContainer = c
	gowid.RunWidgetCallbacks(w.Callbacks, SavedContainer{}, app, w)
}

func (w *Widget) OnOpenClose(f gowid.IWidgetChangedCallback) {
	gowid.AddWidgetCallback(w.Callbacks, OpenCloseCB{}, f)
}

func (w *Widget) RemoveOnOpenClose(f gowid.IIdentity) {
	gowid.RemoveWidgetCallback(w.Callbacks, OpenCloseCB{}, f)
}

func (w *Widget) UserInput(ev interface{}, size gowid.IRenderSize, focus gowid.Selector, app gowid.IApp) bool {
	return UserInput(w, ev, size, focus, app)
}

func (w *Widget) Open(container gowid.ISettableComposite, width gowid.IWidgetDimension, app gowid.IApp) {
	Open(w, container, width, app)
}

func (w *Widget) Close(app gowid.IApp) {
	Close(w, app)
}

// Open the dialog at the top-level of the widget hierarchy which is the App - it itself
// is an IComposite
//
func (w *Widget) OpenGlobally(width gowid.IWidgetDimension, app gowid.IApp) {
	w.Open(app, width, app)
}

func (w *Widget) Width() gowid.IWidgetDimension {
	return w.SavedContainer().SubWidget().(*overlay.Widget).Width()
}

func (w *Widget) SetWidth(d gowid.IWidgetDimension, app gowid.IApp) {
	w.SavedContainer().SubWidget().(*overlay.Widget).SetWidth(d, app)
}

func (w *Widget) Height() gowid.IWidgetDimension {
	return w.SavedContainer().SubWidget().(*overlay.Widget).Height()
}

func (w *Widget) SetHeight(d gowid.IWidgetDimension, app gowid.IApp) {
	w.SavedContainer().SubWidget().(*overlay.Widget).SetHeight(d, app)
}

func (w *Widget) SetContentWidth(d gowid.IWidgetDimension, app gowid.IApp) {
	w.contentWrapper.D = d
}

//''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''

func Close(w IWidget, app gowid.IApp) {
	w.SavedContainer().SetSubWidget(w.SavedSubWidget(), app)
	w.SetOpen(false, app)
}

func Open(w IOpenExt, container gowid.ISettableComposite, width gowid.IWidgetDimension, app gowid.IApp) {
	OpenExt(w, container, width, gowid.RenderFlow{}, app)
}

type IOpenExt interface {
	gowid.IWidget
	SetSavedSubWidget(w gowid.IWidget, app gowid.IApp)
	SetSavedContainer(c gowid.ISettableComposite, app gowid.IApp)
	SetOpen(open bool, app gowid.IApp)
	SetContentWidth(w gowid.IWidgetDimension, app gowid.IApp)
}

func OpenExt(w IOpenExt, container gowid.ISettableComposite, width gowid.IWidgetDimension, height gowid.IWidgetDimension, app gowid.IApp) {
	ov := overlay.New(w, container.SubWidget(),
		gowid.VAlignMiddle{}, height, // Intended to mean use as much vertical space as you need
		gowid.HAlignMiddle{}, width)

	if _, ok := width.(gowid.IRenderFixed); ok {
		w.SetContentWidth(gowid.RenderFixed{}, app) // fixed or weight:1, ratio:0.5
	} else {
		w.SetContentWidth(gowid.RenderWithWeight{W: 1}, app) // fixed or weight:1, ratio:0.5
	}
	w.SetSavedSubWidget(container.SubWidget(), app)
	w.SetSavedContainer(container, app)
	container.SetSubWidget(ov, app)
	w.SetOpen(true, app)
}

func UserInput(w IWidget, ev interface{}, size gowid.IRenderSize, focus gowid.Selector, app gowid.IApp) bool {
	var res bool
	if w.IsOpen() {
		if evk, ok := ev.(*tcell.EventKey); ok && (evk.Key() == tcell.KeyCtrlC || evk.Key() == tcell.KeyEsc) && w.EscapeCloses() {
			w.GetNoFunction().Changed(app, w)
		} else {
			// discard result, we always handle it
			gowid.UserInputIfSelectable(w.SubWidget(), ev, size, focus, app)
		}
		res = true
	} else {
		res = gowid.UserInputIfSelectable(w.SubWidget(), ev, size, focus, app)
	}
	return res
}

//======================================================================

type Maximizer struct {
	Maxed  bool
	Width  gowid.IWidgetDimension
	Height gowid.IWidgetDimension
}

func (m *Maximizer) Maximize(w IWidget, app gowid.IApp) bool {
	if m.Maxed {
		return false
	}
	m.Width = w.Width()
	m.Height = w.Height()
	w.SetWidth(gowid.RenderWithRatio{R: 1.0}, app)
	w.SetHeight(gowid.RenderWithRatio{R: 1.0}, app)
	m.Maxed = true
	return true
}

func (m *Maximizer) Unmaximize(w IWidget, app gowid.IApp) bool {
	if !m.Maxed {
		return false
	}
	w.SetWidth(m.Width, app)
	w.SetHeight(m.Height, app)
	m.Maxed = false
	return true
}

//======================================================================
// Local Variables:
// mode: Go
// fill-column: 110
// End:
