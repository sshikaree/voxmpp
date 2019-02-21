package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"log"
	"time"

	"github.com/BurntSushi/toml"

	xmpp "github.com/sshikaree/go-xmpp2"
)

const (
	XMPP_CONNECT_DELAY time.Duration = 2 * time.Minute
	PING_INTERVAL      time.Duration = 15 * time.Minute
)

type App struct {
	*xmpp.Client
	options        xmpp.Options
	IncomingBuffer chan []byte
	OutgoingBuffer chan []byte
}

func NewApp() *App {
	config := Config{}
	if _, err := toml.DecodeFile("config.toml", &config); err != nil {
		log.Fatal(err)
	}

	a := App{}
	a.options = xmpp.Options{
		Host:          config.XMPP.Host,
		User:          config.XMPP.User,
		Password:      config.XMPP.Password,
		NoTLS:         config.XMPP.NoTLS,
		Debug:         config.XMPP.Debug,
		Status:        "chat",
		StatusMessage: "Hi there!",
	}

	a.IncomingBuffer = make(chan []byte, 50)
	a.OutgoingBuffer = make(chan []byte, 50)

	return &a
}

func (a *App) ConnectToXMPPServer() error {
	var err error
	xmpp.DefaultConfig = tls.Config{
		ServerName:         a.options.Host,
		InsecureSkipVerify: false,
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
		log.Println(err)
		time.Sleep(XMPP_CONNECT_DELAY)
	}
}

func (a *App) ParseXMPPMessage(msg *xmpp.Message) {
	// log.Println("Recieved message!")
	switch msg.Type {
	case "chat":
		log.Println(msg.Body)

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
