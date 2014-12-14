package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	gmail "github.com/google/google-api-go-client/gmail/v1"
)

// Flags
var (
	query   = flag.String("query", "in:inbox is:unread to:me category:personal", "Gmail search expression, for messages to send reply to.")
	message = flag.String("message", "This is an autoresponse to your referenced email. I received your email while out of the office. I am now slowly going through my inbox. If you wish to ensure your email is looked at, please reply now.", "Message body to send in reply.")
	start   = flag.String("start_date", "", "Start date for message query (YYYY/MM/DD).")
	end     = flag.String("end_date", "", "Start date for message query (YYYY/MM/DD).")
	prompt  = flag.Bool("prompt", true, "Prompt y/n for sending replies.")

	// Oauth stuff.
	clientId     = flag.String("clientid", "", "OAuth Client ID.  If non-empty, overrides --clientid_file")
	clientIdFile = flag.String("clientid_file", "clientid.dat",
		"Name of a file containing just the project's OAuth Client ID from https://code.google.com/apis/console/")
	secret     = flag.String("secret", "", "OAuth Client Secret.  If non-empty, overrides --secret_file")
	secretFile = flag.String("secret_file", "clientsecret.dat",
		"Name of a file containing just the project's OAuth Client Secret from https://code.google.com/apis/console/")
	cacheToken = flag.Bool("cachetoken", true, "cache the OAuth token")
)

// message is used to store the message ID and subject of thread leaf messages that we must compose replies to.
type msg struct {
	Id       string
	Subject  string
	ThreadId string
}

func main() {
	flag.Parse()
	if *message == "" {
		log.Fatalf("Missing required flag --message")
	}

	config.Scope = gmail.MailGoogleComScope
	config.ClientId = valueOrFileContents(*clientId, *clientIdFile)
	config.ClientSecret = valueOrFileContents(*secret, *secretFile)
	c := getOAuthClient(config)

	// Initialize Gmail client.
	svc, err := gmail.New(c)
	if err != nil {
		log.Fatalf("Unable to create Gmail service: %v", err)
	}

	// Get the user's email address (for the From header).
	p, err := svc.Users.GetProfile("me").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve user profile: %v", err)
	}

	// Get messages matching our filter.
	if *start != "" {
		*query = fmt.Sprintf("%s after:%s", *query, *start)
	}
	if *end != "" {
		*query = fmt.Sprintf("%s before:%s", *query, *end)
	}
	ts := make([]string, 0) // Thread IDs.
	var page string = ""
	for true {
		req := svc.Users.Threads.List("me").Q(*query)
		if page != "" {
			req = req.PageToken(page)
		}
		r, err := req.Do()
		if err != nil {
			log.Fatalf("Unable to retrieve messages: %v", err)
		}
		for _, t := range r.Threads {
			ts = append(ts, t.Id)
		}

		page = r.NextPageToken
		if len(r.Threads) == 0 {
			break
		}
		if page == "" {
			break
		}
	}
	log.Printf("Processing %v threads...\n", len(ts))
	// Go through all the threads and get their messages.
	for _, id := range ts {
		t, err := svc.Users.Threads.Get("me", id).Do()
		if err != nil {
			log.Fatalf("Unable to retrieve messages: %v", err)
		}
		// Figure out leaf messages we have to reply to.
		ls := make(map[string]msg) // sender -> message ID
		// Assume t.Messages is sorted by time...
		for i := len(t.Messages) - 1; i >= 0; i-- {
			m := t.Messages[i]
			from := ""
			subj := ""
			mid := ""
			for _, h := range m.Payload.Headers {
				switch h.Name {
				case "From":
					from = h.Value
				case "Subject":
					subj = h.Value
				case "Message-ID":
					mid = h.Value
				}
			}
			if strings.Contains(from, p.EmailAddress) {
				// Stop at last point in thread where we replied.
				break
			}
			if _, ok := ls[from]; !ok {
				// Reply only once per sender.
				ls[from] = msg{Id: mid, Subject: subj, ThreadId: t.Id}
			}
		}
		// Reader to read from user input.
		reader := bufio.NewReader(os.Stdin)

		// For each leaf message:
		for f, m := range ls {
			// Make a reply.
			msg := "From: %s\r\nTo: %s\r\nIn-Reply-To: %s\r\nSubject: Re: %s\r\n\r\n%s"
			msg = fmt.Sprintf(msg, p.EmailAddress, f, m.Id, m.Subject, *message)
			r := gmail.Message{Raw: encodeWeb64String(msg)}
			fmt.Printf("Reply to message from %s: %s\n", f, m.Subject)
			send := false
			if *prompt {
				for true {
					fmt.Printf("Send response? (y/n)  ")
					val, err := reader.ReadString('\n')
					if err != nil {
						log.Fatalf("unable to scan input: %v", err)
					}
					val = strings.TrimSpace(val)
					switch val {
					case "y", "Y":
						send = true
					case "n", "N":
						send = false
					default:
						fmt.Printf("Please enter 'y' or 'n'.\n")
						continue
					}
					break
				}
			} else {
				send = true
			}
			// Send it.
			if send {
				if _, err := svc.Users.Messages.Send("me", &r).Do(); err != nil {
					log.Printf("Error sending reply to MessageID %s: %v", m.Id, err)

				}
			}
		}

	}
}

func encodeWeb64String(b string) string {
	s := base64.URLEncoding.EncodeToString([]byte(b))
	var i = len(s) - 1
	for s[i] == '=' {
		i--
	}
	return s[0 : i+1]
}
