package main

import (
	"encoding/xml"
)

const (
	NSVOXMPP = "http://jabber.org/protocol/ibb/voxmpp"
)

type VOXMPPOpen struct {
	XMLName   xml.Name `xml:"http://jabber.org/protocol/ibb/voxmpp open"`
	BlockSize int      `xml:"block-size,attr"`
	SID       string   `xml:"sid,attr"`
	Stanza    string   `xml:"stanza,attr,omitempty"` // iq or message
}

type VOXMPPData struct {
	XMLName xml.Name `xml:"http://jabber.org/protocol/ibb/voxmpp data"`
	Seq     int      `xml:"seq,attr"`
	SID     string   `xml:"sid,attr"`
	Payload []byte   `xml:",chardata"`
}

type VOXMPPCLose struct {
	XMLName xml.Name `xml:"http://jabber.org/protocol/ibb/voxmpp close"`
	SID     string   `xml:"sid,attr"`
}
