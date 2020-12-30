package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/disintegration/imaging"
	"gopkg.in/alexcesaro/quotedprintable.v2"
	"html/template"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type MailSenderInstance struct {
	smtpParams    *SmtpParams
	deliver       *Deliver
	emailsChannel chan string
	emailsCount   int
	emailsSended  int
}

func ConfigureMailSenderInstance(smtpParams *SmtpParams, deliver *Deliver) *MailSenderInstance {
	return &MailSenderInstance{
		smtpParams,
		deliver,
		make(chan string),
		0,
		0,
	}
}

func (sender *MailSenderInstance) PreparePreviewMailBody() string {
	mtemplate := GetTemplateFromDB(sender.deliver.MailTemplateName)
	decodedString := DecodeFile(mtemplate.TemplateFile)
	fileString := string(decodedString)

	funcMap := template.FuncMap{
		"safe": func(s string) template.URL {
			return template.URL(s)
		},
		"comment": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	t := template.Must(template.New("").Funcs(funcMap).Parse(fileString))

	buf := &bytes.Buffer{}
	if err := t.Execute(buf, &sender.deliver.MailNews); err != nil {
		Error.Println(err)
		return ""
	}

	return buf.String()
}

func (sender *MailSenderInstance) PrepareMailBody() string {
	if !StaticDirectory(sender.deliver.MailDate) {
		Error.Println("Unable to create directory for static files.")
		return ""
	}

	for _, newsBlock := range sender.deliver.MailNews.NewsLettersBlocks {
		for _, news := range newsBlock.NewsLetters {
			if news.Image != "" {
				filePath := sender.WriteStaticImage(news.Image, news.NewsNumber, newsBlock.BlockNumber)
				if filePath != "" {
					news.Image = "https://mailer.quattuor.ru/" + filePath
				}
			}
		}
	}

	mtemplate := GetTemplateFromDB(sender.deliver.MailTemplateName)
	decodedString := DecodeFile(mtemplate.TemplateFile)
	fileString := string(decodedString)

	funcMap := template.FuncMap{
		"safe": func(s string) template.URL {
			return template.URL(s)
		},
		"comment": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	t := template.Must(template.New("").Funcs(funcMap).Parse(fileString))

	buf := &bytes.Buffer{}
	if err := t.Execute(buf, &sender.deliver.MailNews); err != nil {
		Error.Println(err)
		return ""
	}

	return buf.String()
}

func (sender *MailSenderInstance) WriteStaticImage(imageb64 string, filename string, blockid string) string {
	var prefix = regexp.MustCompile(`.*image/`)
	var suffix = regexp.MustCompile(`;.*`)
	getExt := prefix.ReplaceAllString(imageb64, "")
	getExt2 := suffix.ReplaceAllString(getExt, "")
	if getExt2 == "" {
		Error.Println("Unknown mime type:", getExt)
		return ""
	}
	imageAsBytes := DecodeFile(imageb64)
	imageReader := bytes.NewReader(imageAsBytes)
	img, err := imaging.Decode(imageReader)
	if err != nil {
		Error.Println(err)
		return ""
	}
	tWidth, tHeight := SelectTemplateImagesParams(sender.deliver.MailTemplateName)
	dstImage := imaging.Resize(img, tWidth, tHeight, imaging.Lanczos)
	filePath := "static/" + sender.deliver.MailDate + "/" + blockid + filename + "." + getExt2
	if err := imaging.Save(dstImage, filePath); err != nil {
		Error.Println(err)
		return ""
	}
	if err := os.Chmod(filePath, 755); err != nil {
		Error.Println(err)
		return ""
	}
	return filePath
}

func SelectTemplateImagesParams(templateName string) (int, int) {
	switch templateName {
	case "pochtabank":
		return 200, 150
	case "minoboron":
		return 160, 120
	default:
		return 160, 120
	}
}

func StaticDirectory(deliverId string) bool {
	dirName := "static/" + deliverId
	src, err := os.Stat(dirName)
	if os.IsNotExist(err) {
		errDir := os.MkdirAll(dirName, 0755)
		if errDir != nil {
			Error.Println(err)
		}
		Info.Println("Creating directory", dirName)
		return true
	}
	if src.Mode().IsRegular() {
		Error.Println(dirName, "is exist as file.")
		return false
	}
	return true
}

func (sender *MailSenderInstance) RunDelivery() bool {
	/**
	threadsToInt, err := strconv.Atoi(sender.smtpParams.Threads)
	if err != nil{
		Error.Println(err)
		return false
	}
	*/
	deliveryGroups := GlobalGroups
	var deliveryAddrSlice []string
	for _, groupToDeliver := range sender.deliver.RecipientGroups {
		for _, rcptgroup := range deliveryGroups {
			if groupToDeliver == rcptgroup.GroupName {
				for email, _ := range rcptgroup.GroupMails {
					deliveryAddrSlice = append(deliveryAddrSlice, email)
				}
			}
		}
	}

	attachMap := make(map[string][]byte)

	for _, attach := range sender.deliver.Attachments {
		decodedAttach := DecodeFile(attach.Body)
		attachMap[attach.Name] = decodedAttach
	}

	mailBody := sender.PrepareMailBody()

	if mailBody != "" {
		sender.emailsCount = len(deliveryAddrSlice)
		for _, email := range deliveryAddrSlice {
			//go sender.SendEmail(email, &attachMap, mailBody)
			sender.SendEmail(email, &attachMap, mailBody)
		}
		sender.deliver.IsDelivered = true
		UpdateDeliveryState(sender.deliver.MailDate, sender.deliver.IsDelivered)
		return true
	} else {
		Error.Println(sender.deliver.MailDate, "body is not exist.")
		return false
	}
}

func (sender *MailSenderInstance) SendEmail(recipientAddr string, attachments *map[string][]byte, body string) {
	sender.emailsSended++
	message := NewMessage()
	message.SetHeader("From", sender.deliver.SenderMail)
	message.SetHeader("To", recipientAddr)
	message.SetHeader("Subject", sender.deliver.MailTheme)
	message.SetBody("text/html", body)
	for attachName, attachBytes := range *attachments {
		attach := CreateFile(attachName, attachBytes)
		if attach != nil {
			message.Attach(attach)
		}
	}

	addr := sender.smtpParams.SMTPServer + ":" + sender.smtpParams.SMTPPort
	auth := LoginAuth(sender.smtpParams.SMTPLogin, sender.smtpParams.SMTPPass, sender.smtpParams.SMTPServer)

	mclient := NewCustomMailer(addr, auth)

	if err := mclient.Send(message); err != nil {
		Error.Println(err)
		return
	}
}

// Message represents an email.
type Message struct {
	header      header
	parts       []part
	attachments []*File
	embedded    []*File
	charset     string
	encoding    Encoding
	hEncoder    *quotedprintable.HeaderEncoder
}

type header map[string][]string

type part struct {
	contentType string
	body        *bytes.Buffer
}

// NewMessage creates a new message. It uses UTF-8 and quoted-printable encoding
// by default.
func NewMessage(settings ...MessageSetting) *Message {
	msg := &Message{
		header:   make(header),
		charset:  "UTF-8",
		encoding: QuotedPrintable,
	}

	msg.applySettings(settings)

	var e quotedprintable.Encoding
	if msg.encoding == Base64 {
		e = quotedprintable.B
	} else {
		e = quotedprintable.Q
	}
	msg.hEncoder = e.NewHeaderEncoder(msg.charset)

	return msg
}

func (msg *Message) applySettings(settings []MessageSetting) {
	for _, s := range settings {
		s(msg)
	}
}

// A MessageSetting can be used as an argument in NewMessage to configure an
// email.
type MessageSetting func(msg *Message)

// SetCharset is a message setting to set the charset of the email.
//
// Example:
//
//	msg := gomail.NewMessage(SetCharset("ISO-8859-1"))
func SetCharset(charset string) MessageSetting {
	return func(msg *Message) {
		msg.charset = charset
	}
}

// SetEncoding is a message setting to set the encoding of the email.
//
// Example:
//
//	msg := gomail.NewMessage(SetEncoding(gomail.Base64))
func SetEncoding(enc Encoding) MessageSetting {
	return func(msg *Message) {
		msg.encoding = enc
	}
}

// Encoding represents a MIME encoding scheme like quoted-printable or base64.
type Encoding string

const (
	// QuotedPrintable represents the quoted-printable encoding as defined in
	// RFC 2045.
	QuotedPrintable Encoding = "quoted-printable"
	// Base64 represents the base64 encoding as defined in RFC 2045.
	Base64 Encoding = "base64"
	// Unencoded can be used to avoid encoding the body of an email. The headers
	// will still be encoded using quoted-printable encoding.
	Unencoded Encoding = "8bit"
)

// SetHeader sets a value to the given header field.
func (msg *Message) SetHeader(field string, value ...string) {
	for i := range value {
		value[i] = encodeHeader(msg.hEncoder, value[i])
	}
	msg.header[field] = value
}

// SetHeaders sets the message headers.
//
// Example:
//
//	msg.SetHeaders(map[string][]string{
//		"From":    {"alex@example.com"},
//		"To":      {"bob@example.com", "cora@example.com"},
//		"Subject": {"Hello"},
//	})
func (msg *Message) SetHeaders(h map[string][]string) {
	for k, v := range h {
		msg.SetHeader(k, v...)
	}
}

// SetAddressHeader sets an address to the given header field.
func (msg *Message) SetAddressHeader(field, address, name string) {
	msg.header[field] = []string{msg.FormatAddress(address, name)}
}

// FormatAddress formats an address and a name as a valid RFC 5322 address.
func (msg *Message) FormatAddress(address, name string) string {
	buf := getBuffer()
	defer putBuffer(buf)

	if !quotedprintable.NeedsEncoding(name) {
		quote(buf, name)
	} else {
		var n string
		if hasSpecials(name) {
			n = encodeHeader(quotedprintable.B.NewHeaderEncoder(msg.charset), name)
		} else {
			n = encodeHeader(msg.hEncoder, name)
		}
		buf.WriteString(n)
	}
	buf.WriteString(" <")
	buf.WriteString(address)
	buf.WriteByte('>')

	return buf.String()
}

// SetDateHeader sets a date to the given header field.
func (msg *Message) SetDateHeader(field string, date time.Time) {
	msg.header[field] = []string{msg.FormatDate(date)}
}

// FormatDate formats a date as a valid RFC 5322 date.
func (msg *Message) FormatDate(date time.Time) string {
	return date.Format(time.RFC1123Z)
}

// GetHeader gets a header field.
func (msg *Message) GetHeader(field string) []string {
	return msg.header[field]
}

// DelHeader deletes a header field.
func (msg *Message) DelHeader(field string) {
	delete(msg.header, field)
}

// SetBody sets the body of the message.
func (msg *Message) SetBody(contentType, body string) {
	msg.parts = []part{
		part{
			contentType: contentType,
			body:        bytes.NewBufferString(body),
		},
	}
}

// AddAlternative adds an alternative body to the message. Commonly used to
// send HTML emails that default to the plain text version for backward
// compatibility.
//
// Example:
//
//	msg.SetBody("text/plain", "Hello!")
//	msg.AddAlternative("text/html", "<p>Hello!</p>")
//
// More info: http://en.wikipedia.org/wiki/MIME#Alternative
func (msg *Message) AddAlternative(contentType, body string) {
	msg.parts = append(msg.parts,
		part{
			contentType: contentType,
			body:        bytes.NewBufferString(body),
		},
	)
}

// GetBodyWriter gets a writer that writes to the body. It can be useful with
// the templates from packages text/template or html/template.
//
// Example:
//
//	w := msg.GetBodyWriter("text/plain")
//	t := template.Must(template.New("example").Parse("Hello {{.}}!"))
//	t.Execute(w, "Bob")
func (msg *Message) GetBodyWriter(contentType string) io.Writer {
	buf := new(bytes.Buffer)
	msg.parts = append(msg.parts,
		part{
			contentType: contentType,
			body:        buf,
		},
	)

	return buf
}

// A File represents a file that can be attached or embedded in an email.
type File struct {
	Name      string
	MimeType  string
	Content   []byte
	ContentID string
}

// OpenFile opens a file on disk to create a gomail.File.
func OpenFile(filename string) (*File, error) {
	content, err := readFile(filename)
	if err != nil {
		return nil, err
	}

	f := CreateFile(filepath.Base(filename), content)

	return f, nil
}

// CreateFile creates a gomail.File from the given name and content.
func CreateFile(name string, content []byte) *File {
	mimeType := mime.TypeByExtension(filepath.Ext(name))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return &File{
		Name:     name,
		MimeType: mimeType,
		Content:  content,
	}
}

// Attach attaches the files to the email.
func (msg *Message) Attach(f ...*File) {
	if msg.attachments == nil {
		msg.attachments = f
	} else {
		msg.attachments = append(msg.attachments, f...)
	}
}

// Embed embeds the images to the email.
//
// Example:
//
//	f, err := gomail.OpenFile("/tmp/image.jpg")
//	if err != nil {
//		panic(err)
//	}
//	msg.Embed(f)
//	msg.SetBody("text/html", `<img src="cid:image.jpg" alt="My image" />`)
func (msg *Message) Embed(image ...*File) {
	if msg.embedded == nil {
		msg.embedded = image
	} else {
		msg.embedded = append(msg.embedded, image...)
	}
}

// Stubbed out for testing.
var readFile = ioutil.ReadFile

func quote(buf *bytes.Buffer, text string) {
	buf.WriteByte('"')
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' || text[i] == '"' {
			buf.WriteByte('\\')
		}
		buf.WriteByte(text[i])
	}
	buf.WriteByte('"')
}

func hasSpecials(text string) bool {
	for i := 0; i < len(text); i++ {
		switch c := text[i]; c {
		case '(', ')', '<', '>', '[', ']', ':', ';', '@', '\\', ',', '.', '"':
			return true
		}
	}

	return false
}

func encodeHeader(enc *quotedprintable.HeaderEncoder, value string) string {
	if !quotedprintable.NeedsEncoding(value) {
		return value
	}

	return enc.Encode(value)
}

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

func putBuffer(buf *bytes.Buffer) {
	if buf.Len() > 1024 {
		return
	}
	buf.Reset()
	bufPool.Put(buf)
}

func (m *Mailer) getSendMailFunc(ssl bool) SendMailFunc {
	return func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		var c smtpClient
		var err error
		if ssl {
			c, err = sslDial(addr, m.host, m.config)
		} else {
			c, err = starttlsDial(addr, m.config)
		}
		if err != nil {
			return err
		}
		defer c.Close()

		if a != nil {
			if ok, _ := c.Extension("AUTH"); ok {
				if err = c.Auth(a); err != nil {
					return err
				}
			}
		}

		if err = c.Mail(from); err != nil {
			return err
		}

		for _, addr := range to {
			if err = c.Rcpt(addr); err != nil {
				return err
			}
		}

		w, err := c.Data()
		if err != nil {
			return err
		}
		_, err = w.Write(msg)
		if err != nil {
			return err
		}
		err = w.Close()
		if err != nil {
			return err
		}

		return c.Quit()
	}
}

func sslDial(addr, host string, config *tls.Config) (smtpClient, error) {
	conn, err := initTLS("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	return newClient(conn, host)
}

func starttlsDial(addr string, config *tls.Config) (smtpClient, error) {
	c, err := initSMTP(addr)
	if err != nil {
		return c, err
	}

	if ok, _ := c.Extension("STARTTLS"); ok {
		return c, c.StartTLS(config)
	}

	return c, nil
}

var initSMTP = func(addr string) (smtpClient, error) {
	return smtp.Dial(addr)
}

var initTLS = func(network, addr string, config *tls.Config) (*tls.Conn, error) {
	return tls.Dial(network, addr, config)
}

var newClient = func(conn net.Conn, host string) (smtpClient, error) {
	return smtp.NewClient(conn, host)
}

type smtpClient interface {
	Extension(string) (bool, string)
	StartTLS(*tls.Config) error
	Auth(smtp.Auth) error
	Mail(string) error
	Rcpt(string) error
	Data() (io.WriteCloser, error)
	Quit() error
	Close() error
}

type Mailer struct {
	addr   string
	host   string
	config *tls.Config
	auth   smtp.Auth
	send   SendMailFunc
}

// A MailerSetting can be used in a mailer constructor to configure it.
type MailerSetting func(m *Mailer)

// SetSendMail allows to set the email-sending function of a mailer.
//
// Example:
//
//	myFunc := func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
//		// Implement your email-sending function similar to smtp.SendMail
//	}
//	mailer := gomail.NewMailer("host", "user", "pwd", 465, SetSendMail(myFunc))
func SetSendMail(s SendMailFunc) MailerSetting {
	return func(m *Mailer) {
		m.send = s
	}
}

// SetTLSConfig allows to set the TLS configuration used to connect the SMTP
// server.
func SetTLSConfig(c *tls.Config) MailerSetting {
	return func(m *Mailer) {
		m.config = c
	}
}

// A SendMailFunc is a function to send emails with the same signature than
// smtp.SendMail.
type SendMailFunc func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

// NewMailer returns a mailer. The given parameters are used to connect to the
// SMTP server via a PLAIN authentication mechanism.
func NewMailer(host string, username string, password string, port int, settings ...MailerSetting) *Mailer {
	return NewCustomMailer(
		fmt.Sprintf("%s:%d", host, port),
		smtp.PlainAuth("", username, password, host),
		settings...,
	)
}

// NewCustomMailer creates a mailer with the given authentication mechanism.
//
// Example:
//
//	gomail.NewCustomMailer("host:587", smtp.CRAMMD5Auth("username", "secret"))
func NewCustomMailer(addr string, auth smtp.Auth, settings ...MailerSetting) *Mailer {
	// Error is not handled here to preserve backward compatibility
	host, port, _ := net.SplitHostPort(addr)

	m := &Mailer{
		addr: addr,
		host: host,
		auth: auth,
	}

	for _, s := range settings {
		s(m)
	}

	if m.config == nil {
		m.config = &tls.Config{ServerName: host}
	}
	if m.send == nil {
		m.send = m.getSendMailFunc(port == "465")
	}

	return m
}

// Send sends the emails to all the recipients of the message.
func (m *Mailer) Send(msg *Message) error {
	message := msg.Export()

	from, err := getFrom(message)
	if err != nil {
		return err
	}
	recipients, bcc, err := getRecipients(message)
	if err != nil {
		return err
	}

	h := flattenHeader(message, "")
	body, err := ioutil.ReadAll(message.Body)
	if err != nil {
		return err
	}

	mail := append(h, body...)
	if err := m.send(m.addr, m.auth, from, recipients, mail); err != nil {
		return err
	}

	for _, to := range bcc {
		h = flattenHeader(message, to)
		mail = append(h, body...)
		if err := m.send(m.addr, m.auth, from, []string{to}, mail); err != nil {
			return err
		}
	}

	return nil
}

func flattenHeader(msg *mail.Message, bcc string) []byte {
	buf := getBuffer()
	defer putBuffer(buf)

	for field, value := range msg.Header {
		if field != "Bcc" {
			buf.WriteString(field)
			buf.WriteString(": ")
			buf.WriteString(strings.Join(value, ", "))
			buf.WriteString("\r\n")
		} else if bcc != "" {
			for _, to := range value {
				if strings.Contains(to, bcc) {
					buf.WriteString(field)
					buf.WriteString(": ")
					buf.WriteString(to)
					buf.WriteString("\r\n")
				}
			}
		}
	}
	buf.WriteString("\r\n")

	return buf.Bytes()
}

func getFrom(msg *mail.Message) (string, error) {
	from := msg.Header.Get("Sender")
	if from == "" {
		from = msg.Header.Get("From")
		if from == "" {
			return "", errors.New("mailer: invalid message, \"From\" field is absent")
		}
	}

	return parseAddress(from)
}

func getRecipients(msg *mail.Message) (recipients, bcc []string, err error) {
	for _, field := range []string{"Bcc", "To", "Cc"} {
		if addresses, ok := msg.Header[field]; ok {
			for _, addr := range addresses {
				switch field {
				case "Bcc":
					bcc, err = addAdress(bcc, addr)
				default:
					recipients, err = addAdress(recipients, addr)
				}
				if err != nil {
					return recipients, bcc, err
				}
			}
		}
	}

	return recipients, bcc, nil
}

func addAdress(list []string, addr string) ([]string, error) {
	addr, err := parseAddress(addr)
	if err != nil {
		return list, err
	}
	for _, a := range list {
		if addr == a {
			return list, nil
		}
	}

	return append(list, addr), nil
}

func parseAddress(field string) (string, error) {
	a, err := mail.ParseAddress(field)
	if a == nil {
		return "", err
	}

	return a.Address, err
}

// Export converts the message into a net/mail.Message.
func (msg *Message) Export() *mail.Message {
	w := newMessageWriter(msg)

	if msg.hasMixedPart() {
		w.openMultipart("mixed")
	}

	if msg.hasRelatedPart() {
		w.openMultipart("related")
	}

	if msg.hasAlternativePart() {
		w.openMultipart("alternative")
	}
	for _, part := range msg.parts {
		h := make(map[string][]string)
		h["Content-Type"] = []string{part.contentType + "; charset=" + msg.charset}
		h["Content-Transfer-Encoding"] = []string{string(msg.encoding)}

		w.write(h, part.body.Bytes(), msg.encoding)
	}
	if msg.hasAlternativePart() {
		w.closeMultipart()
	}

	w.addFiles(msg.embedded, false)
	if msg.hasRelatedPart() {
		w.closeMultipart()
	}

	w.addFiles(msg.attachments, true)
	if msg.hasMixedPart() {
		w.closeMultipart()
	}

	return w.export()
}

func (msg *Message) hasMixedPart() bool {
	return (len(msg.parts) > 0 && len(msg.attachments) > 0) || len(msg.attachments) > 1
}

func (msg *Message) hasRelatedPart() bool {
	return (len(msg.parts) > 0 && len(msg.embedded) > 0) || len(msg.embedded) > 1
}

func (msg *Message) hasAlternativePart() bool {
	return len(msg.parts) > 1
}

// messageWriter helps converting the message into a net/mail.Message
type messageWriter struct {
	header     map[string][]string
	buf        *bytes.Buffer
	writers    [3]*multipart.Writer
	partWriter io.Writer
	depth      uint8
}

func newMessageWriter(msg *Message) *messageWriter {
	// We copy the header so Export does not modify the message
	header := make(map[string][]string, len(msg.header)+2)
	for k, v := range msg.header {
		header[k] = v
	}

	if _, ok := header["Mime-Version"]; !ok {
		header["Mime-Version"] = []string{"1.0"}
	}
	if _, ok := header["Date"]; !ok {
		header["Date"] = []string{msg.FormatDate(now())}
	}

	return &messageWriter{header: header, buf: new(bytes.Buffer)}
}

// Stubbed out for testing.
var now = time.Now

func (w *messageWriter) openMultipart(mimeType string) {
	w.writers[w.depth] = multipart.NewWriter(w.buf)
	contentType := "multipart/" + mimeType + "; boundary=" + w.writers[w.depth].Boundary()

	if w.depth == 0 {
		w.header["Content-Type"] = []string{contentType}
	} else {
		h := make(map[string][]string)
		h["Content-Type"] = []string{contentType}
		w.createPart(h)
	}
	w.depth++
}

func (w *messageWriter) createPart(h map[string][]string) {
	// No need to check the error since the underlying writer is a bytes.Buffer
	w.partWriter, _ = w.writers[w.depth-1].CreatePart(h)
}

func (w *messageWriter) closeMultipart() {
	if w.depth > 0 {
		w.writers[w.depth-1].Close()
		w.depth--
	}
}

func (w *messageWriter) addFiles(files []*File, isAttachment bool) {
	for _, f := range files {
		h := make(map[string][]string)
		h["Content-Type"] = []string{f.MimeType + "; name=\"" + f.Name + "\""}
		h["Content-Transfer-Encoding"] = []string{string(Base64)}
		if isAttachment {
			h["Content-Disposition"] = []string{"attachment; filename=\"" + f.Name + "\""}
		} else {
			h["Content-Disposition"] = []string{"inline; filename=\"" + f.Name + "\""}
			if f.ContentID != "" {
				h["Content-ID"] = []string{"<" + f.ContentID + ">"}
			} else {
				h["Content-ID"] = []string{"<" + f.Name + ">"}
			}
		}

		w.write(h, f.Content, Base64)
	}
}

func (w *messageWriter) write(h map[string][]string, body []byte, enc Encoding) {
	w.writeHeader(h)
	w.writeBody(body, enc)
}

func (w *messageWriter) writeHeader(h map[string][]string) {
	if w.depth == 0 {
		for field, value := range h {
			w.header[field] = value
		}
	} else {
		w.createPart(h)
	}
}

func (w *messageWriter) writeBody(body []byte, enc Encoding) {
	var subWriter io.Writer
	if w.depth == 0 {
		subWriter = w.buf
	} else {
		subWriter = w.partWriter
	}

	// The errors returned by writers are not checked since these writers cannot
	// return errors.
	if enc == Base64 {
		writer := base64.NewEncoder(base64.StdEncoding, newBase64LineWriter(subWriter))
		writer.Write(body)
		writer.Close()
	} else if enc == Unencoded {
		subWriter.Write(body)
	} else {
		writer := quotedprintable.NewEncoder(newQpLineWriter(subWriter))
		writer.Write(body)
	}
}

func (w *messageWriter) export() *mail.Message {
	return &mail.Message{Header: w.header, Body: w.buf}
}

// As required by RFC 2045, 6.7. (page 21) for quoted-printable, and
// RFC 2045, 6.8. (page 25) for base64.
const maxLineLen = 76

// base64LineWriter limits text encoded in base64 to 76 characters per line
type base64LineWriter struct {
	w       io.Writer
	lineLen int
}

func newBase64LineWriter(w io.Writer) *base64LineWriter {
	return &base64LineWriter{w: w}
}

func (w *base64LineWriter) Write(p []byte) (int, error) {
	n := 0
	for len(p)+w.lineLen > maxLineLen {
		w.w.Write(p[:maxLineLen-w.lineLen])
		w.w.Write([]byte("\r\n"))
		p = p[maxLineLen-w.lineLen:]
		n += maxLineLen - w.lineLen
		w.lineLen = 0
	}

	w.w.Write(p)
	w.lineLen += len(p)

	return n + len(p), nil
}

// qpLineWriter limits text encoded in quoted-printable to 76 characters per
// line
type qpLineWriter struct {
	w       io.Writer
	lineLen int
}

func newQpLineWriter(w io.Writer) *qpLineWriter {
	return &qpLineWriter{w: w}
}

func (w *qpLineWriter) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		// If the text is not over the limit, write everything
		if len(p) < maxLineLen-w.lineLen {
			w.w.Write(p)
			w.lineLen += len(p)
			return n + len(p), nil
		}

		i := bytes.IndexAny(p[:maxLineLen-w.lineLen+2], "\n")
		// If there is a newline before the limit, write the end of the line
		if i != -1 && (i != maxLineLen-w.lineLen+1 || p[i-1] == '\r') {
			w.w.Write(p[:i+1])
			p = p[i+1:]
			n += i + 1
			w.lineLen = 0
			continue
		}

		// Quoted-printable text must not be cut between an equal sign and the
		// two following characters
		var toWrite int
		if maxLineLen-w.lineLen-2 >= 0 && p[maxLineLen-w.lineLen-2] == '=' {
			toWrite = maxLineLen - w.lineLen - 2
		} else if p[maxLineLen-w.lineLen-1] == '=' {
			toWrite = maxLineLen - w.lineLen - 1
		} else {
			toWrite = maxLineLen - w.lineLen
		}

		// Insert the newline where it is needed
		w.w.Write(p[:toWrite])
		w.w.Write([]byte("=\r\n"))
		p = p[toWrite:]
		n += toWrite
		w.lineLen = 0
	}

	return n, nil
}

type loginAuth struct {
	username string
	password string
	host     string
}

// LoginAuth returns an Auth that implements the LOGIN authentication mechanism.
func LoginAuth(username, password, host string) smtp.Auth {
	return &loginAuth{username, password, host}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS {
		advertised := false
		for _, mechanism := range server.Auth {
			if mechanism == "LOGIN" {
				advertised = true
				break
			}
		}
		if !advertised {
			return "", nil, errors.New("gomail: unencrypted connection")
		}
	}
	if server.Name != a.host {
		return "", nil, errors.New("gomail: wrong host name")
	}
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	command := strings.ToLower(strings.TrimSuffix(string(fromServer), ":"))
	switch command {
	case "username":
		return []byte(fmt.Sprintf("%s", a.username)), nil
	case "password":
		return []byte(fmt.Sprintf("%s", a.password)), nil
	default:
		return nil, fmt.Errorf("gomail: unexpected server challenge: %s", command)
	}
}
