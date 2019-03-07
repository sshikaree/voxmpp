package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	xmpp "github.com/sshikaree/go-xmpp2"
)

// TODO"
// - Redirect os.Stdout to textView

type UI struct {
	*tview.Application
	textView *tview.TextView
	input    *tview.InputField

	btnCall *tview.Button
	btnExit *tview.Button

	flex1 *tview.Flex
	flex2 *tview.Flex

	incomingModal   *tview.Modal
	activeCallModal *tview.Modal
	callModal       *tview.Modal

	pages *tview.Pages

	core *App
}

func NewUI(app *App) *UI {
	var ui UI
	ui.Application = tview.NewApplication()
	ui.core = app

	ui.textView = tview.NewTextView()
	ui.textView.SetBorder(true)
	ui.textView.SetTitle("Welcome to Voxmpp!")

	ui.input = tview.NewInputField().
		SetLabel("Enter JID: ").
		SetFieldWidth(50)

	ui.btnCall = tview.NewButton("Call")
	ui.btnCall.SetBorder(true)
	ui.btnExit = tview.NewButton("Exit")
	ui.btnExit.SetBorder(true)

	ui.incomingModal = tview.NewModal()
	ui.incomingModal.AddButtons([]string{"Accept", "Reject"})

	ui.activeCallModal = tview.NewModal()
	ui.activeCallModal.AddButtons([]string{"Stop"})

	ui.callModal = tview.NewModal()
	ui.callModal.AddButtons([]string{"Cancel"})

	ui.flex1 = tview.NewFlex()
	ui.flex1.SetDirection(tview.FlexRow)

	ui.flex2 = tview.NewFlex()
	ui.flex2.SetDirection(tview.FlexColumn)
	ui.flex2.SetBorder(true)

	ui.flex2.AddItem(ui.input, 55, 1, true)
	ui.flex2.AddItem(nil, 0, 2, false)
	ui.flex2.AddItem(ui.btnCall, 7, 1, true)
	ui.flex2.AddItem(nil, 1, 1, false)
	ui.flex2.AddItem(ui.btnExit, 7, 1, true)

	ui.flex1.AddItem(ui.textView, 0, 6, false)
	ui.flex1.AddItem(ui.flex2, 5, 1, true)

	ui.pages = tview.NewPages()
	ui.pages.AddPage("main_page", ui.flex1, true, true)
	ui.pages.AddPage("incoming_modal", ui.incomingModal, true, false)
	ui.pages.AddPage("active_call_modal", ui.activeCallModal, true, false)
	ui.pages.AddPage("call_modal", ui.callModal, true, false)

	ui.SetRoot(ui.pages, true)

	// Handlers
	ui.input.SetDoneFunc(func(key tcell.Key) {
		ui.SetFocus(ui.btnCall)
	})

	ui.btnCall.SetSelectedFunc(func() {
		fmt.Fprintf(ui.textView, "Calling %s... \n", ui.input.GetText())
		// ui.pages.ShowPage("call_modal")
		go ui.core.CallMsg(ui.input.GetText())
	})
	ui.btnCall.SetBlurFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			ui.SetFocus(ui.btnExit)
		case tcell.KeyBacktab:
			ui.SetFocus(ui.input)
		case tcell.KeyEsc:
			ui.SetFocus(ui.input)
		}
	})

	ui.btnExit.SetSelectedFunc(func() {
		ui.Stop()
	})
	ui.btnExit.SetBlurFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab, tcell.KeyEsc:
			ui.SetFocus(ui.input)
		case tcell.KeyBacktab:
			ui.SetFocus(ui.btnCall)
		}
	})

	return &ui
}

// ShowIncomingModal shows modal window with incoming call
// TODO:
// - should we pass copy of msg???
func (ui *UI) ShowIncomingModal(msg xmpp.Message) {
	fmt.Fprintf(ui.textView, "%s: Incoming call from %s...", time.Now().Local(), msg.From)
	ui.incomingModal.SetText("Incoming call from " + msg.From)
	ui.incomingModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonIndex == 0 {
			// Accept
			ui.ShowActiveCallModal(msg)
			ui.core.AcceptCallMsg(&msg)

		} else {
			// Reject
			ui.core.RejectCallMsg(&msg)

		}
		ui.pages.HidePage("incoming_modal")
	})

	ui.pages.ShowPage("incoming_modal")
}

func (ui *UI) HideIncomingModal() {
	ui.pages.HidePage("incoming_modal")
}

func (ui *UI) ShowActiveCallModal(msg xmpp.Message) {
	ui.activeCallModal.SetText("Connected to " + msg.From)
	ui.activeCallModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		ui.core.RejectCallMsg(&msg)
		ui.pages.HidePage("active_call_modal")
	})
	ui.pages.ShowPage("active_call_modal")
}

func (ui *UI) HideActiveCallModal() {
	ui.pages.HidePage("active_call_modal")
}

func (ui *UI) ShowCallModal(msg xmpp.Message) {
	ui.callModal.SetText("Calling " + ui.input.GetText() + "...")
	ui.callModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		//ui.core.pending.Pop(msg.ID)
		ui.core.AbortOutgoingCall(&msg)
		ui.pages.HidePage("call_modal")
		fmt.Fprintln(ui.textView, " canceled.")
	})
	ui.pages.ShowPage("call_modal")
}

func (ui *UI) HideCallModal() {
	ui.pages.HidePage("call_modal")
}
