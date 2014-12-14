package main

import (
	"flag"
	"fmt"
	"log"

	"code.google.com/p/goauth2/oauth"

	gmail "github.com/google/google-api-go-client/gmail/v1"
)

var config = &oauth.Config{
	ClientId:     "", // Set by --clientid or --clientid_file
	ClientSecret: "", // Set by --secret or --secret_file
	Scope:        "", // filled in per-API
	AuthURL:      "https://accounts.google.com/o/oauth2/auth",
	TokenURL:     "https://accounts.google.com/o/oauth2/token",
}

// Flags
var (
	query   = flag.String("query", "in:inbox is:unread to:me category:personal", "Gmail search expression, for messages to send reply to.")
	message = flag.String("message", "", "Message to send in reply.")
	start   = flag.String("start_date", "", "Start date for message query (YYYY/MM/DD).")
	end     = flag.String("end_date", "", "Start date for message query (YYYY/MM/DD).")

	// Oauth stuff.
	clientId     = flag.String("clientid", "", "OAuth Client ID.  If non-empty, overrides --clientid_file")
	clientIdFile = flag.String("clientid_file", "clientid.dat",
		"Name of a file containing just the project's OAuth Client ID from https://code.google.com/apis/console/")
	secret     = flag.String("secret", "", "OAuth Client Secret.  If non-empty, overrides --secret_file")
	secretFile = flag.String("secret_file", "clientsecret.dat",
		"Name of a file containing just the project's OAuth Client Secret from https://code.google.com/apis/console/")
	cacheToken = flag.Bool("cachetoken", true, "cache the OAuth token")
)

func main() {
	flag.Parse()
	config.Scope = gmail.MailGoogleComScope
	config.ClientId = valueOrFileContents(*clientId, *clientIdFile)
	config.ClientSecret = valueOrFileContents(*secret, *secretFile)
	c := getOAuthClient(config)

	// Initialize Gmail client.
	svc, err := gmail.New(c)
	if err != nil {
		log.Fatalf("Unable to create Gmail service: %v", err)
	}

	// Get messages matching our filter.
	if start != nil {
		*query = fmt.Sprintf("%s after:%s", *query, *start)
	}
	if end != nil {
		*query = fmt.Sprintf("%s before:%s", *query, *end)
	}
	req := svc.Users.Threads.List("me").Q(*query)
	r, err := req.Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v", err)
	}

	log.Printf("Processing %v threads...\n", len(r.Threads))
}
