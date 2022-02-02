package main

import (
	"log"
	"os"
	"time"
	"fmt"
	"strings"
	"os/exec"
	"io/ioutil"
	"bufio"
	//
	"github.com/sendgrid/go-gmime/gmime"
	_ "github.com/proglottis/gpgme"
)

const (
	CRLF = "\r\n"
	EDITOR = "nvim"
)

func _log() {
	log.SetPrefix("epistula ")
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.LUTC | log.Lshortfile)
	if f, err := os.OpenFile("/tmp/c.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		log.Fatal(err)
	} else {
		os.Stderr = f
	}
	log.SetOutput(os.Stderr)
}

func main() {
	// log
	_log()
	log.Printf("main")
	//
	config := NewConfig()
	// The Idea is as follows: the composeser
	// - is called with all information in its arguments like --from, --to, --subject, --cc, --bcc, ...
	var meta_to, meta_reply_to, meta_from, meta_cc, meta_bcc, meta_subject, meta_message_id, content_text string
	for i:=1;i<len(os.Args);i++ {
		if strings.HasPrefix(os.Args[i], "--") {
			x := strings.Split(os.Args[i][2:], "=")
			switch x[0] {
			case "bcc": meta_bcc = x[1]
			case "cc": meta_cc = x[1]
			case "from": meta_from = x[1]
			case "message-id": meta_message_id = x[1]
			case "reply-to": meta_reply_to = x[1]
			case "subject": meta_subject = x[1]
			case "to": meta_to = x[1]
			case "text":
				if fh, err := os.Open(x[1]); err != nil {
					log.Fatal(err)
				} else {
					defer fh.Close()
					if data, err := ioutil.ReadAll(bufio.NewReader(fh)); err != nil {
						log.Fatal(err)
					} else {
						content_text = string(data)
					}
				}
				defer os.Remove(x[1]) // clean up
			default:
				log.Fatal(fmt.Sprintf("wrong arg: %s", os.Args[i]))
			}
		} else {
			log.Fatal(fmt.Sprintf("wrong arg: %s", os.Args[i]))
		}
	}
	if meta_reply_to == "" {
		meta_reply_to = meta_from
	}
	// - composes an email via gmime
	var buffer []byte
	date_string := time.Now().Format(time.RFC1123Z)
	// go-gmime doesnt support creation of envelopes or parts in envelopes yet.
	// so we create an empty dummy email and modify the elements after parsing
	// that
	if message, err := gmime.Parse(
		"Date: " + date_string + CRLF +
		"From: " + config.user_name + " <" + config.user_primary_email + ">" + CRLF +
		CRLF +
		CRLF); err != nil {
		log.Fatal(err)
	} else {
		message.ClearAddress("From")
		message.ParseAndAppendAddresses("From", config.user_name + " <" + config.user_primary_email + ">")
		message.ParseAndAppendAddresses("To", meta_reply_to) // TODO how to add an empty "To:", .. ?
		message.ParseAndAppendAddresses("To", meta_to) // if multiple to: exist reply to all of them
		// TODO remove myself
		message.ParseAndAppendAddresses("Cc", meta_cc)
		message.ParseAndAppendAddresses("Bcc", meta_bcc)
		message.SetSubject(meta_subject)
		message.SetHeader("X-Epistula-Status", "I am not done")
		message.SetHeader("X-Epistula-Comment", "This is your MUA talking to you. Add attachments as headerfield like below. Dont destroy the mail structure, if the outcome cant be parsed you will thrown into your editor again to fix it. Change the Status to not contain 'not'. Add a 'abort' to abort sending (editings lost).")
		message.SetHeader("X-Epistula-Attachment", "#sample entry#")
		if content_text != "" {
			content_text = "> " + strings.ReplaceAll(content_text, "\n", "\n> ")
		}
		if err := message.Walk(func (part *gmime.Part) error {
			if part.IsText() && part.ContentType() == "text/plain" {
				part.SetText(content_text)
			}
			return nil
		}); err != nil {
			log.Fatal(err)
		}
		if b, err := message.Export(); err != nil {
			log.Fatal(err)
		} else {
			buffer = b
		}
	}
	// - exports it to a temp file
	var tempfilename string
	if f, err := os.CreateTemp("", "epistula-composer-"); err != nil {
		log.Fatal(err)
	} else {
		if _, err := f.Write(buffer); err != nil {
			log.Fatal(err)
		}
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
		tempfilename = f.Name()
	}
	defer os.Remove(tempfilename)
	// - execs the editor and waits for its termination
	// set terminal title
	title := "Epistula Composer: " + config.user_name + " <" + config.user_primary_email + ">" + " to " + meta_reply_to
	os.Stdout.Write([]byte("\x1b]1;"+title+"\a\x1b]2;"+title+"\a"))
	//
	var message *gmime.Envelope
	done := false
	abort := false
	for !done {
		if EDITOR, err := exec.LookPath(EDITOR); err == nil {
			var procAttr os.ProcAttr
			procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
			if proc, err := os.StartProcess(EDITOR, []string{EDITOR, "+", tempfilename}, &procAttr); err == nil {
				proc.Wait()
			}
		}
	// - parses the file via gmime
		message = parseFile(tempfilename)
		status := message.Header("X-Epistula-Status")
		done = !strings.Contains(status, "not")
		abort = strings.Contains(status, "abort")
	}
	if abort {
		// the user flagged the message to be aborted
		os.Exit(0)
	}
	message.RemoveHeader("X-Epistula-Status")
	message.RemoveHeader("X-Epistula-Comment")
	message.RemoveHeader("X-Epistula-Attachment") // TODO add attachment
	message.ParseAndAppendAddresses("Reply-To", config.user_primary_email)
	message.SetHeader("MIME-Version", "1.0")
	message.SetHeader("User-Agent", "Epistula")
	message.SetHeader("Content-Type", "text/plain; charset=utf-8")
	message.SetHeader("Content-Transfer-Encoding", "quoted-printable")
	message.SetHeader("In-Reply-To", meta_message_id)
	// message.SetHeader("Content-ID", )
	// message.SetHeader("Message-ID", )
	// message.SetHeader("References", )
	// message.SetHeader("Return-Path", )
	// message.SetHeader("Thread-Topic", )
	// - retreives the desired keys
	// - encrypts the file via gpgme
	// - sends the email
	if b, err := message.Export(); err != nil {
		log.Fatal(err)
	} else {
		buffer = b
	}
	cmd := exec.Command("sendmail", "-t", )
	if stdin, err := cmd.StdinPipe(); err != nil {
		log.Fatal(err)
	} else {
		go func() {
			defer stdin.Close()
			stdin.Write(buffer)
		}()
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Fatal(err)
		} else {
			log.Printf("%s\n", out)
		}
	}
	// - saves the email in maildir and kicks off notmuch new, tag 'sent'
	cmd = exec.Command("notmuch", "insert", "+sent", )
	if stdin, err := cmd.StdinPipe(); err != nil {
		log.Fatal(err)
	} else {
		go func() {
			defer stdin.Close()
			stdin.Write(buffer)
		}()
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Fatal(err)
		} else {
			log.Printf("%s\n", out)
		}
	}
}

func parseFile(filename string) *gmime.Envelope {
	if fh, err := os.Open(filename); err != nil {
		return nil
	} else {
		defer fh.Close()
		if data, err := ioutil.ReadAll(bufio.NewReader(fh)); err != nil {
			return nil
		} else {
			if envelope, err := gmime.Parse(string(data)); err != nil {
				return nil
			} else {
				return envelope
			}
		}
	}
	return nil
}

