package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
	xmpp "github.com/sshikaree/go-xmpp2"
)

const (
	XMPP_CONNECT_DELAY time.Duration = 1 * time.Minute
	PING_INTERVAL      time.Duration = 15 * time.Minute

	CALL_TIMEOUT = 25 * time.Second
)

type RemoteJID struct {
	jid string
	sync.Mutex
}

func (r *RemoteJID) Get() string {
	r.Lock()
	defer r.Unlock()
	return r.jid
}

func (r *RemoteJID) Set(jid string) {
	r.Lock()
	r.jid = jid
	r.Unlock()
}

// Call holds response an done channel on request with certain id
type Call struct {
	Req   *xmpp.Message
	Resp  *xmpp.Message
	Done  chan bool
	Error error
}

func NewCall(req *xmpp.Message) *Call {
	return &Call{
		Req:  req,
		Resp: &xmpp.Message{},
		Done: make(chan bool),
	}
}

// PendingCalls holds pending calls map
type PendingCalls struct {
	sync.Mutex
	calls map[string]*Call
}

// Push adds new call to pending calls map
func (pc *PendingCalls) Push(msg *xmpp.Message) *Call {
	call := NewCall(msg)
	pc.Lock()
	pc.calls[msg.ID] = call
	pc.Unlock()
	return call
}

// Pop returns call with given id and deletes it from pending calls.
// If id does not exist nil is returned.
func (pc *PendingCalls) Pop(id string) *Call {
	pc.Lock()
	defer pc.Unlock()
	call, ok := pc.calls[id]
	if !ok {
		return nil
	}
	delete(pc.calls, id)
	return call
}

func (pc *PendingCalls) Done(response *xmpp.Message, e error) {
	pc.Lock()
	defer pc.Unlock()
	call, ok := pc.calls[response.ID]
	if !ok {
		return
	}
	call.Resp = response
	call.Error = e
	call.Done <- true
}

type App struct {
	*xmpp.Client
	options        xmpp.Options
	IncomingBuffer chan []byte
	OutgoingBuffer chan []byte
	RemoteJID      RemoteJID
	pending        *PendingCalls
	// scanner        *bufio.Scanner
	// inputReader *InputReader
	ui *UI

	// ExitCh chan bool
}

func NewApp(jid *string, password *string, notls bool, debug bool) *App {
	server := strings.Split(*jid, "@")
	if len(server) < 2 {
		log.Fatal("Wrong JID")
	}

	a := App{}
	a.options = xmpp.Options{
		Host:          server[1],
		User:          *jid,
		Password:      *password,
		NoTLS:         notls,
		Debug:         debug,
		Status:        "chat",
		StatusMessage: "Hi there!",
	}

	a.IncomingBuffer = make(chan []byte, 50)
	a.OutgoingBuffer = make(chan []byte, 50)

	a.pending = &PendingCalls{
		calls: make(map[string]*Call),
	}

	// a.ExitCh = make(chan bool)
	// a.scanner = bufio.NewScanner(os.Stdin)

	// a.inputReader = &InputReader{}
	// a.inputReader.InputReaderStart()

	a.ui = NewUI(&a)
	return &a
}

func (a *App) ConnectToXMPPServer() error {
	var err error
	if a.options.NoTLS == false {
		xmpp.DefaultConfig = tls.Config{
			ServerName:         a.options.Host,
			InsecureSkipVerify: false,
		}
	}

	a.Client, err = a.options.NewClient()

	return err
}

func (a *App) ConnectXMPPAndRetry() {
	for {
		err := a.ConnectToXMPPServer()
		if err == nil {
			break
		}
		log.Println("Error connecting server:", err)
		time.Sleep(XMPP_CONNECT_DELAY)
	}
}

// ParseXMPPMessage parses incoming message stanza
func (a *App) ParseXMPPMessage(msg *xmpp.Message) {
	// log.Println("Recieved message!")
	switch msg.Type {
	// skip for now
	case "chat":
		fmt.Fprintf(a.ui.textView, "(%v) %s: %s\n", time.Now(), msg.From, msg.Body)
		// fmt.Println(msg.Body)

	// call rejected
	case "error":
		a.RemoteJID.Set("")
		a.pending.Done(msg, errors.New("Call was rejected"))

	// call accepted
	case "result":
		// log.Println("result recieved!")
		a.pending.Done(msg, nil)

	case "voxmpp":
		// log.Println("voxmpp message recieved!")
		// log.Println(msg)
		if len(msg.OtherElements) < 1 {
			return
		}
		if msg.OtherElements[0].XMLName.Space != NSVOXMPP {
			return
		}
		switch msg.OtherElements[0].XMLName.Local {
		// request to open channel
		case "open":
			a.ui.QueueUpdateDraw(func() {
				a.ui.ShowIncomingModal(*msg)
			})
			// TODO: send "wait.." message?
			// fmt.Printf("Incoming call from %s. Pick up? (Y/n): ", msg.From)
			// for {
			// 	switch a.inputReader.Get() { //a.scanner.Text() {
			// 	case "y", "Y":
			// 		fmt.Printf("Accepted.\n~> ")
			// 		a.AcceptCallMsg(msg)
			// 		return
			// 	case "n", "N":
			// 		fmt.Printf("Rejected.\n~> ")
			// 		a.RejectCallMsg(msg)
			// 		return
			// 	default:
			// 		fmt.Print("Enter 'y', 'n' or press 'enter' for default(y): ")
			// 		// a.scanner.Scan()
			// 	}
			// }

		// data package
		case "data":
			// connection is not established
			if a.RemoteJID.Get() == "" {
				return
			}
			// data := VOXMPPData{}
			if len(msg.OtherElements) < 1 {
				return
			}
			bindata, err := DecodePayload(msg.OtherElements[0].InnerXML)
			if err != nil {
				log.Println(err)
				return
			}
			a.IncomingBuffer <- bindata

		}

	}

}

// AcceptCall accepsts incoming call
func (a *App) AcceptCallMsg(msg *xmpp.Message) {
	resp := xmpp.Message{}
	resp.To = msg.From
	resp.ID = msg.ID
	resp.Type = "result"

	_, err := a.SendMessage(&resp)
	if err != nil {
		log.Println(err)
	}
	a.RemoteJID.Set(msg.From)

}

// RejectCall rejects incoming call.
// If msg == nil, abort current call.
func (a *App) RejectCallMsg(msg *xmpp.Message) {
	resp := xmpp.Message{}
	if msg != nil {
		resp.To = msg.From
		resp.ID = msg.ID
	} else {
		resp.To = a.RemoteJID.Get()
		u, err := uuid.NewV4()
		if err != nil {
			log.Println(err)
			return
		}
		resp.ID = u.String()
	}

	resp.Type = "error"

	_, err := a.SendMessage(&resp)
	if err != nil {
		log.Println(err)
	}

	a.RemoteJID.Set("")
}

// AbortOutgoingCall aborts call
// It takes copy of original message
func (a *App) AbortOutgoingCall(msg xmpp.Message) {
	msg.Type = "error"
	msg.OtherElements = []xmpp.XMLElement{}
	// msg.Error = &xmpp.Error{}

	// text, _ := xml.Marshal(&msg)
	// fmt.Fprintln(a.ui.textView, string(text))

	a.pending.Pop(msg.ID)

	_, err := a.SendMessage(&msg)
	if err != nil {
		log.Println(err)
	}
	a.RemoteJID.Set("")

}

// Not implemented
func (a *App) ParseIQ(iq *xmpp.IQ) {
	log.Printf("%+v\n", iq)
}

// SendChunk endcodes into base64 and sends chunk of data
func (a *App) SendChunk(chunk []byte, to string) error {
	if chunk == nil || len(chunk) == 0 {
		return nil
	}
	chunk_base64 := make([]byte, base64.StdEncoding.EncodedLen(len(chunk)))
	base64.StdEncoding.Encode(chunk_base64, chunk)

	msg := xmpp.Message{}
	msg.To = to
	msg.Type = "voxmpp"
	msg.OtherElements = []xmpp.XMLElement{
		{
			XMLName: xml.Name{
				Space: NSVOXMPP,
				Local: "data",
			},
			InnerXML: chunk_base64,
		},
	}

	_, err := a.SendMessage(&msg)
	return err
}

// CalIQ sends call request using iq stanza
// Not implemented
func (a *App) CallIQ(jid string) error {
	query := xmpp.Query{}
	query.XMLName.Space = NSVOXMPP
	query.XMLName.Local = "open"
	iq := xmpp.IQ{}
	iq.ID = "123" // TODO: Generate randomly
	iq.To = jid
	iq.Type = "set"
	iq.InnerElement = query

	// b, err := xml.Marshal(&iq)
	// if err != nil {
	// 	log.Println(err)
	// }
	// fmt.Println(string(b))

	_, err := a.SendIQ(&iq)
	return err

}

// CallMsg sends call request using message stanza
func (a *App) CallMsg(jid string) error {
	msg := xmpp.Message{}
	msg.To = jid
	msg.Type = "voxmpp"
	u, err := uuid.NewV4()
	if err != nil {
		return err
	}
	msg.ID = u.String() // TODO: Generate randomly
	msg.OtherElements = []xmpp.XMLElement{
		{
			XMLName: xml.Name{
				Space: NSVOXMPP,
				Local: "open",
			},
			// InnerXML: []byte("hello"),
		},
	}

	// b, err := xml.Marshal(&msg)
	// if err != nil {
	// 	log.Println(err)
	// }
	// fmt.Println(string(b))
	a.ui.QueueUpdateDraw(func() {
		a.ui.ShowCallModal(msg)
	})
	call := a.pending.Push(&msg)

	_, err = a.SendMessage(&msg)
	if err != nil {
		a.pending.Pop(msg.ID)
		return err
	}
	// TODO:
	// - Start calling sound
	// - Display calling message
	// fmt.Printf("Calling %s (timeout %v)... ", jid, CALL_TIMEOUT)
	select {
	case <-call.Done:

	case <-time.After(CALL_TIMEOUT):
		// TODO:
		// - Stop calling
		a.ui.QueueUpdateDraw(func() {
			a.ui.HideCallModal()
		})
		a.AbortOutgoingCall(msg)
		err = errors.New("Request timeout")
		call.Error = err
		a.pending.Pop(msg.ID)
		a.RemoteJID.Set("")
		return err
	}
	a.ui.QueueUpdateDraw(func() {
		a.ui.HideCallModal()
	})
	if call.Error != nil {
		fmt.Printf("Error connecting %s: %s", jid, call.Error)
		return call.Error
	}
	a.ui.QueueUpdateDraw(func() {
		a.ui.ShowActiveCallModal(msg)
	})
	a.RemoteJID.Set(jid)
	// fmt.Printf("Connection with %s established!", jid)
	return nil
}
