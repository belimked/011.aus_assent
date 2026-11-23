package main

import (
	"log"
	"os"
	"strings"
)

type rexConfig struct {
	BaseURL, AppID, Email, Password, AccountID, WriteAccountID string
	Read, Write                                                bool
	Missing                                                    []string
	ReadReason, WriteReason                                    string
}

func loadRexConfig() rexConfig {
	c := rexConfig{BaseURL: os.Getenv("REX_BASE_URL"), AppID: os.Getenv("REX_APP_ID"), Email: os.Getenv("REX_EMAIL"), Password: os.Getenv("REX_PASSWORD"), AccountID: os.Getenv("REX_ACCOUNT_ID"), WriteAccountID: os.Getenv("REX_WRITE_ACCOUNT_ID")}
	for _, pair := range [][2]string{{"REX_BASE_URL", c.BaseURL}, {"REX_APP_ID", c.AppID}, {"REX_EMAIL", c.Email}, {"REX_PASSWORD", c.Password}, {"REX_ACCOUNT_ID", c.AccountID}} {
		if strings.TrimSpace(pair[1]) == "" {
			c.Missing = append(c.Missing, pair[0])
		}
	}
	if len(c.Missing) > 0 {
		c.ReadReason = "missing " + strings.Join(c.Missing, ", ")
		c.WriteReason = c.ReadReason
	} else {
		if os.Getenv("AXLEO_REX_READ") == "1" {
			c.Read = true
		} else {
			c.ReadReason = "AXLEO_REX_READ is not 1"
		}
		switch {
		case os.Getenv("AXLEO_REX_WRITE") != "1":
			c.WriteReason = "AXLEO_REX_WRITE is not 1"
		case c.WriteAccountID != c.AccountID:
			c.WriteReason = "REX_WRITE_ACCOUNT_ID does not match REX_ACCOUNT_ID"
		default:
			c.Write = true
		}
	}
	return c
}
func (c rexConfig) canRead() bool  { return c.Read }
func (c rexConfig) canWrite() bool { return c.Write }
func (c rexConfig) logStartup() {
	if os.Getenv("AXLEO_REX_ENABLED") != "" {
		log.Print("AXLEO_REX_ENABLED is deprecated and ignored")
	}
	if len(c.Missing) > 0 {
		log.Printf("Rex disabled: %s", c.ReadReason)
	}
	if !c.Read {
		log.Printf("Rex reads disabled: %s", c.ReadReason)
	}
	if !c.Write {
		log.Printf("Rex writes disabled: %s", c.WriteReason)
	}
}
