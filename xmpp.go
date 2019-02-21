package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	xmpp "github.com/sshikaree/go-xmpp2"
)

const (
	XMPP_CONNECT_DELAY time.Duration = 2 * time.Minute
	PING_INTERVAL      time.Duration = 15 * time.Minute
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

type App struct {
	*xmpp.Client
	options        xmpp.Options
	IncomingBuffer chan []byte
	OutgoingBuffer chan []byte
	RemoteJID      RemoteJID
}

func NewApp(jid *string, password *string, notls bool, debug bool) *App {
	// config := Config{}
	// if _, err := toml.DecodeFile("config.toml", &config); err != nil {
	// 	log.Fatal(err)
	// }
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

func (a *App) ParseLine(line string) {
	tokens := strings.Split(line, " ")

	switch tokens[0] {
	case "/call":
		if len(tokens) < 2 {
			fmt.Println("Not enough parameters")
			return
		}
		a.RemoteJID.Set(tokens[1])
	case "/stop":
		a.RemoteJID.Set("")
	case "/exit":
		os.Exit(0)
	default:
		if len(tokens) < 2 {
			fmt.Println("Not enough parameters")
			return
		}
		msg := xmpp.Message{}
		msg.To = tokens[0]
		msg.Type = "chat"
		msg.Body = tokens[1]
		a.SendMessage(&msg)
	}
}

func (a *App) ParseXMPPMessage(msg *xmpp.Message) {
	// log.Println("Recieved message!")
	switch msg.Type {
	case "chat":
		fmt.Println(msg.Body)

	case "ibb":
		// log.Println("Recieved IBB!")
		// log.Println(msg.Body)
		data := xmpp.IBBData{}
		if err := xml.Unmarshal([]byte(msg.Body), &data); err != nil {
			log.Println(err)
			return
		}
		bindata, err := base64.StdEncoding.DecodeString(string(data.Payload))
		if err != nil {
			log.Println(err)
			return
		}
		a.IncomingBuffer <- bindata
	}

}

func (a *App) SendChunk(chunk []byte, to string) error {
	if chunk == nil || len(chunk) == 0 {
		return nil
	}
	// var chunk_base64 []byte
	// base64.StdEncoding.Encode(chunk_base64, chunk)
	base64_string := base64.StdEncoding.EncodeToString(chunk)
	body, err := xml.Marshal(xmpp.XMLElement{
		XMLName: xml.Name{
			Space: "http://jabber.org/protocol/ibb",
			Local: "data",
		},
		InnerXML: []byte(base64_string),
	})
	if err != nil {
		return err
	}

	msg := xmpp.Message{}
	msg.To = to
	msg.Type = "ibb"
	msg.Body = string(body)

	_, err = a.SendMessage(&msg)
	return err
}
