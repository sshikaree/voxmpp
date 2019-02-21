package main

type Config struct {
	XMPP xmppConf `toml:"xmpp"`
}

type xmppConf struct {
	Host     string
	User     string
	Password string
	NoTLS    bool
	Debug    bool
}
