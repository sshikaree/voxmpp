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

	ui *UI
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
	// text, _ := xml.Marshal(&msg)
	// a.ui.QueueUpdateDraw(func() {
	// 	fmt.Fprintf(a.ui.textView, "Message received: %s\n", string(text))
	// })

	switch msg.Type {
	// skip for now
	case "chat":
		fmt.Fprintf(a.ui.textView, "(%v) %s: %s\n", time.Now(), msg.From, msg.Body)

	case "voxmpp":
		// error received
		// call rejected
		if msg.Error != nil {
			if msg.Error.XMLName.Space != NSVOXMPP {
				return
			}
			a.RemoteJID.Set("")
			a.ui.QueueUpdateDraw(func() {
				// 	fmt.Fprintf(a.ui.textView, "Error message received: %s\n", string(text))
				a.ui.HideActiveCallModal()
			})
			a.pending.Done(msg, errors.New("Call was rejected"))
		}

		if len(msg.OtherElements) < 1 {
			return
		}
		if msg.OtherElements[0].XMLName.Space != NSVOXMPP {
			return
		}

		switch msg.OtherElements[0].XMLName.Local {

		// call accepted
		case "result":
			a.pending.Done(msg, nil)

		// request to open channel
		case "open":
			// add call to calls pool
			call := a.pending.Push(msg)

			go func() {
				select {
				case <-call.Done:

				case <-time.After(CALL_TIMEOUT):
					// a.RejectCallMsg(msg)
				}

				a.pending.Pop(msg.ID)
				a.ui.QueueUpdateDraw(func() {
					a.ui.HideIncomingModal()
				})
			}()

			a.ui.QueueUpdateDraw(func() {
				a.ui.ShowIncomingModal(*msg)
			})

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

// AcceptCall accepts incoming call
func (a *App) AcceptCallMsg(msg *xmpp.Message) {
	resp := xmpp.Message{}
	resp.To = msg.From
	resp.ID = msg.ID
	resp.Type = "voxmpp"
	resp.OtherElements = []xmpp.XMLElement{
		{
			XMLName: xml.Name{
				Space: NSVOXMPP,
				Local: "result",
			},
		},
	}

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
	resp.Type = "voxmpp"
	resp.Error = &xmpp.Error{
		XMLName: xml.Name{
			Space: NSVOXMPP,
			Local: "error",
		},
	}

	_, err := a.SendMessage(&resp)
	if err != nil {
		log.Println(err)
	}

	a.RemoteJID.Set("")
}

// AbortOutgoingCall aborts call
func (a *App) AbortOutgoingCall(msg *xmpp.Message) {
	errMsg := xmpp.Message{}
	errMsg.Type = "voxmpp"
	errMsg.To = msg.To
	errMsg.ID = msg.ID
	errMsg.Error = &xmpp.Error{
		XMLName: xml.Name{
			Space: NSVOXMPP,
			Local: "error",
		},
	}

	// a.pending.Pop(msg.ID)
	a.pending.Done(msg, errors.New("cancelled"))

	_, err := a.SendMessage(&errMsg)
	if err != nil {
		log.Println(err)
	}
	// text, _ := xml.Marshal(&errMsg)
	// a.ui.QueueUpdateDraw(func() {
	// 	fmt.Fprintf(a.ui.textView, "Abort message was sent: %s\n", string(text))
	// })
	a.RemoteJID.Set("")
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
		},
	}

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

	select {
	case <-call.Done:

	case <-time.After(CALL_TIMEOUT):
		a.ui.QueueUpdateDraw(func() {
			a.ui.HideCallModal()
		})
		a.AbortOutgoingCall(&msg)
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
		fmt.Fprintf(a.ui.textView, "Error connecting %s: %s\n", jid, call.Error)
		a.ui.QueueUpdateDraw(func() {
			a.ui.HideCallModal()
		})
		a.pending.Pop(msg.ID)
		return call.Error
	}

	a.ui.QueueUpdateDraw(func() {
		a.ui.ShowActiveCallModal(msg)
	})
	a.RemoteJID.Set(jid)
	a.pending.Pop(msg.ID)
	return nil
}
