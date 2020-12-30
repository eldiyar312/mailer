package main

import (
	"encoding/base64"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	defaultSmtpServer  *SmtpParams
	DBConnectionString string
	GlobalGroupNames   []string
	GlobalGroups       []RcptGroup
	GlobalTemplates    []MailTemplate
	DeliverSave        *Deliver
)

var (
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

func LogInit(
	traceHandle io.Writer,
	infoHandle io.Writer,
	warningHandle io.Writer,
	errorHandle io.Writer) {

	Trace = log.New(traceHandle,
		"TRACE: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Info = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Warning = log.New(warningHandle,
		"WARNING: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

type MailerApp struct {
	Conf             *Configuration
	Router           *mux.Router
	DBConnString     string
	ConnectedClients []*ClientConn
}

type Configuration struct {
	ServicePort string
	DBAddr      string
	DBUser      string
	DBPass      string
	DBName      string
	MailServer  *SmtpParams
}

type SmtpParams struct {
	SMTPServer string
	SMTPPort   string
	SMTPLogin  string
	SMTPPass   string
	Threads    string
}

type ClientConn struct {
	Conn          *websocket.Conn
	IsAvailable   bool
	ClientChannel chan *ClientWSMessage
}

type ClientWSMessage struct {
	Timestamp   string `json:"timestamp"`
	MessageType string `json:"messagetype"`
	MessageBody string `json:"messagebody"`
}

type CustomMailTemplate struct {
	Name string
	Body []byte
}

type MailNewsLetters struct {
	NewsHeader        string
	NewsLettersBlocks []*NewsLettersBlock
}

type NewsLettersBlock struct {
	BlockHeader string        `json:"BlockHeader"`
	BlockNumber string        `json:"BlockNumber"`
	BlockLink   string        `json:"BlockLink"`
	NewsLetters []*NewsLetter `json:"NewsLetters"`
}

type NewsLetter struct {
	NewsNumber string `json:"NewsNumber"`
	Header     string `json:"Header"`
	Annotation string `json:"Annotation"`
	Source     string `json:"Source"`
	Link       string `json:"Link"`
	Image      string `json:"Image"`
	ImageLink  string `json:"ImageLink"`
}

type RcptGroup struct {
	GroupName  string            `json:"GroupName"`
	GroupMails map[string]string `json:"GroupMails"`
}

type RcptInfo struct {
	GroupName string `json:"GroupName"`
	RcptEmail string `json:"RcptEmail"`
	RcptName  string `json:"RcptName"`
}

type MailTemplate struct {
	TemplateFileName string           `json:"TemplateFileName"`
	TemplateFile     string           `json:"TemplateFile"`
	TemplateName     string           `json:"TemplateName"`
	TemplateImages   []*TemplateImage `json:"TemplateImages"`
}

type TemplateImage struct {
	ImageName string `json:"ImageName"`
	ImageBody string `json:"ImageBody"`
}

type Attachment struct {
	Name string `json:"Name"`
	Body string `json:"Body"`
}

type Deliver struct {
	MailDate         string `json:"MailDate"`
	SenderMail       string `json:"SenderMail"`
	SenderName       string `json:"SenderName"`
	MailTemplate     *CustomMailTemplate
	MailTemplateName string           `json:"MailTemplateName"`
	MailTheme        string           `json:"MailTheme"`
	Attachments      []*Attachment    `json:"Attachments"`
	MailNews         *MailNewsLetters `json:"MailNews"`
	MailBody         []byte
	IsConfirmed      bool     `json:"IsConfirmed"`
	ConfirmMail      string   `json:"ConfirmMail"`
	RecipientGroups  []string `json:"RecipientGroups"`
	IsDelivered      bool     `json:"IsDelivered"`
}

type ImageForPreview struct {
	Block      int    `json:"Block"`
	NewsLetter int    `json:"NewsLetter"`
	FileLink   string `json:"FileLink"`
	FileData   string `json:"FileData"`
}

func (mailer *MailerApp) wsHandler(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	ws, err := upgrader.Upgrade(w, r, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		return
	}
	client := &ClientConn{
		ws,
		true,
		make(chan *ClientWSMessage),
	}
	mailer.ConnectedClients = append(mailer.ConnectedClients, client)
	go client.Load()
}

func (clientConn *ClientConn) Load() {
	go clientConn.connReader()
	go clientConn.connWriter()
	go clientConn.connPinger()
}

func (clientConn *ClientConn) connPinger() {
	for clientConn.IsAvailable == true {
		if clientConn.Conn != nil {
			clientConn.ClientChannel <- &ClientWSMessage{
				GetDateString(),
				"Keepalive",
				"",
			}
			time.Sleep(5 * time.Second)
		} else {
			clientConn.IsAvailable = false
		}
	}
}

func (clientConn *ClientConn) connReader() {
	for clientConn.IsAvailable == true {
		getMessage := ClientWSMessage{}
		if clientConn.Conn != nil {
			err := clientConn.Conn.ReadJSON(&getMessage)
			if err != nil {
				clientConn.IsAvailable = false
				break
			}
			switch getMessage.MessageType {
			case "TestMessageType":
				go clientConn.TestWSMessage(&getMessage)
			case "DeliverSave":
				go clientConn.SaveDeliverInfo(&getMessage)
			case "SavedDeliverRequest":
				go clientConn.SavedDeliverRequest(&getMessage)
			case "ClearDeliverRequest":
				go clientConn.ClearDeliverRequest()
			case "DeliverSaveAndSend":
				go clientConn.SaveAndSendDeliver(&getMessage)
			case "ShowDeliver":
				go clientConn.ShowDeliver(&getMessage)
			case "RCPTGroupsRequest":
				go clientConn.RCPTGroupsRequest()
			case "AddressesGroupsRequest":
				go clientConn.AddressesGroupsRequest()
			case "RefreshRCPTGroups":
				//Для апдейта на странице Группы Рассылки
				go GetRCPTGroupNamesFromDB()
			case "HistoryRequest":
				go clientConn.HistoryRequest()
			case "HistoryInstanceRequest":
				go clientConn.HistoryInstanceRequest(&getMessage)
			case "GroupAddRequest":
				go clientConn.AddGroupRequest(&getMessage)
			case "GroupRemoveRequest":
				go clientConn.RemoveGroupRequest(&getMessage)
			case "UserAddRequest":
				go clientConn.UserAddRequest(&getMessage)
			case "UserRemoveRequest":
				go clientConn.UserRemoveRequest(&getMessage)
			case "TemplateAddRequest":
				go clientConn.TemplateAddRequest(&getMessage)
			case "TemplatesRequest":
				go clientConn.TemplatesRequest()
			case "TemplateDeleteRequest":
				go clientConn.TemplatesDeleteRequest(&getMessage)
			case "ImageFromLinkRequest":
				go clientConn.ImageFromLinkRequest(&getMessage)
			default:
				clientConn.Conn.Close()
			}
		} else {
			clientConn.IsAvailable = false
			break
		}
	}
}

func (clientConn *ClientConn) connWriter() {
	for clientConn.IsAvailable == true {
		message := clientConn.CheckMessages()
		if message != nil {
			switch message.MessageType {
			case "Keepalive":
				err := clientConn.Conn.WriteMessage(websocket.PingMessage, []byte("keepalive"))
				if err != nil {
					clientConn.IsAvailable = false
					clientConn.Conn.Close()
					break
				}
			default:
				err := clientConn.Conn.WriteJSON(message)
				if err != nil {
					clientConn.IsAvailable = false
					clientConn.Conn.Close()
					break
				}
			}
		}
	}
}

func (clientConn *ClientConn) CheckMessages() *ClientWSMessage {
	select {
	case message := <-clientConn.ClientChannel:
		return message
	}
}

func (clientConn *ClientConn) TestWSMessage(message *ClientWSMessage) {
	Info.Println(message.MessageType)
	Info.Println(message.MessageBody)
}

func (clientConn *ClientConn) ImageFromLinkRequest(message *ClientWSMessage) {
	var imageData ImageForPreview
	err := json.Unmarshal([]byte(message.MessageBody), &imageData)
	if err != nil {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"ImageFromLinkResponseFail",
			"Невозможно скачать файл по этой ссылке.",
		}
		return
	}

	response, err := http.Get(imageData.FileLink)
	if err != nil {
		Error.Println(err)
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"ImageFromLinkResponseFail",
			"Невозможно скачать файл по этой ссылке.",
		}
		return
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			Error.Println(err)
			clientConn.ClientChannel <- &ClientWSMessage{
				GetDateString(),
				"ImageFromLinkResponseFail",
				"Невозможно скачать файл по этой ссылке.",
			}
			return
		}
		sEnc := base64.StdEncoding.EncodeToString(bodyBytes)
		imageData.FileData = "data:" + response.Header.Get("Content-Type") + ";base64," + sEnc
		sendImage, err := json.Marshal(&imageData)
		if err != nil {
			Error.Println(err)
			clientConn.ClientChannel <- &ClientWSMessage{
				GetDateString(),
				"ImageFromLinkResponseFail",
				"Невозможно скачать файл по этой ссылке.",
			}
			return
		}
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"ImageFromLinkResponseSuccess",
			string(sendImage),
		}
	}
}

func (clientConn *ClientConn) SaveDeliverInfo(message *ClientWSMessage) {
	clientConn.ClientChannel <- &ClientWSMessage{
		GetDateString(),
		"SaveDeliverState",
		"Рассылка сохраняется...",
	}
	var newDeliver Deliver
	err := json.Unmarshal([]byte(message.MessageBody), &newDeliver)
	if err != nil {
		Error.Println(err)
	}
	newDeliver.MailDate = GetDateString()
	newDeliver.SenderMail = defaultSmtpServer.SMTPLogin
	newDeliver.MailNews.NewsHeader = newDeliver.MailTheme
	if SaveDeliverToDB(&newDeliver) {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"SaveDeliverState",
			"Рассылка успешно сохранена.",
		}
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"UnlockSaveButton",
			"true",
		}
	} else {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"SaveDeliverState",
			"При сохранении рассылки произошла ошибка!",
		}
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"UnlockSaveButton",
			"true",
		}
	}
}

func (clientConn *ClientConn) ClearDeliverRequest() {
	var clearDeliver Deliver
	DeliverSave = &clearDeliver
	savedDeliver, err := json.Marshal(&DeliverSave)
	Info.Println(&DeliverSave)
	if err != nil {
		Error.Println("Cannot serialize saved deliver:", err)
		return
	}
	clientConn.ClientChannel <- &ClientWSMessage{
		GetDateString(),
		"SavedDeliverResponse",
		string(savedDeliver),
	}
}

func (clientConn *ClientConn) SavedDeliverRequest(message *ClientWSMessage) {
	savedDeliver, err := json.Marshal(&DeliverSave)
	if err != nil {
		Error.Println("Cannot serialize saved deliver:", err)
		return
	}
	clientConn.ClientChannel <- &ClientWSMessage{
		GetDateString(),
		"SavedDeliverResponse",
		string(savedDeliver),
	}
}

func (clientConn *ClientConn) ShowDeliver(message *ClientWSMessage) {
	var newDeliver Deliver
	err := json.Unmarshal([]byte(message.MessageBody), &newDeliver)
	if err != nil {
		Error.Println(err)
	}
	newDeliver.MailDate = GetDateString()
	newDeliver.SenderMail = defaultSmtpServer.SMTPLogin
	newDeliver.MailNews.NewsHeader = newDeliver.MailTheme
	senderInstance := ConfigureMailSenderInstance(defaultSmtpServer, &newDeliver)
	bodyString := senderInstance.PreparePreviewMailBody()
	clientConn.ClientChannel <- &ClientWSMessage{
		GetDateString(),
		"DeliverPreview",
		bodyString,
	}
}

func (clientConn *ClientConn) SaveAndSendDeliver(message *ClientWSMessage) {
	var newDeliver Deliver
	err := json.Unmarshal([]byte(message.MessageBody), &newDeliver)
	if err != nil {
		Error.Println(err)
	}
	newDeliver.MailDate = GetDateString()
	newDeliver.SenderMail = defaultSmtpServer.SMTPLogin
	newDeliver.MailNews.NewsHeader = newDeliver.MailTheme
	clientConn.ClientChannel <- &ClientWSMessage{
		GetDateString(),
		"SaveDeliverState",
		"Рассылка сохраняется...",
	}
	if SaveDeliverToDB(&newDeliver) {
		senderInstance := ConfigureMailSenderInstance(defaultSmtpServer, &newDeliver)
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"SaveDeliverState",
			"Рассылка отправляется...",
		}
		if senderInstance.RunDelivery() {
			clientConn.ClientChannel <- &ClientWSMessage{
				GetDateString(),
				"SaveDeliverState",
				"Рассылка успешно сохранена и отправлена.",
			}
			clientConn.ClientChannel <- &ClientWSMessage{
				GetDateString(),
				"UnlockSendButton",
				"true",
			}

		} else {
			clientConn.ClientChannel <- &ClientWSMessage{
				GetDateString(),
				"SaveDeliverState",
				"При отправлении рассылки произошла ошибка!",
			}
			clientConn.ClientChannel <- &ClientWSMessage{
				GetDateString(),
				"UnlockSendButton",
				"true",
			}
		}
	} else {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"SaveDeliverState",
			"При сохранении рассылки произошла ошибка!",
		}
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"UnlockSendButton",
			"true",
		}
	}
}

func (clientConn *ClientConn) RCPTGroupsRequest() {
	rgroups := strings.Join(GlobalGroupNames, ",")
	clientConn.ClientChannel <- &ClientWSMessage{
		GetDateString(),
		"RCPTGroupsResponse",
		rgroups,
	}
}

func (clientConn *ClientConn) HistoryRequest() {
	go GetHistoryFromDB(clientConn)
}

func (clientConn *ClientConn) HistoryInstanceRequest(message *ClientWSMessage) {
	go GetHistoryInstance(clientConn, message.MessageBody)
}

func (clientConn *ClientConn) AddGroupRequest(message *ClientWSMessage) {
	if CheckGroupIsExist(message.MessageBody) {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"GroupAddResponse",
			"Группа с таким названием уже существует.",
		}
	} else {
		if !SaveNewGroupNameToDB(message.MessageBody) {
			clientConn.ClientChannel <- &ClientWSMessage{
				GetDateString(),
				"GroupAddResponse",
				"Ошибка добавления группы, нужно смотреть логи.",
			}
		} else {
			clientConn.ClientChannel <- &ClientWSMessage{
				GetDateString(),
				"GroupAddResponse",
				"Группа \"" + message.MessageBody + "\" успешно добавлена.",
			}
		}
	}
}

func (clientConn *ClientConn) RemoveGroupRequest(message *ClientWSMessage) {
	if CheckGroupIsExist(message.MessageBody) {
		if RemoveGroupNameFromDB(message.MessageBody) {
			clientConn.ClientChannel <- &ClientWSMessage{
				GetDateString(),
				"GroupRemoveResponse",
				"Группа \"" + message.MessageBody + "\" удалена.",
			}
		} else {
			clientConn.ClientChannel <- &ClientWSMessage{
				GetDateString(),
				"GroupRemoveResponse",
				"Ошибка удаления группы \"" + message.MessageBody + "\"! Смотреть логи.",
			}
		}
	} else {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"GroupRemoveResponse",
			"Группы \"" + message.MessageBody + "\" не существует.",
		}
	}
}

func (clientConn *ClientConn) UserAddRequest(message *ClientWSMessage) {
	var rcptInfo RcptInfo
	err := json.Unmarshal([]byte(message.MessageBody), &rcptInfo)
	if err != nil {
		Error.Println(err)
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"UserAddResponse",
			"Не удалось добавить адресата из-за ошибки парсинга JSON.",
		}
		return
	}
	if !CheckUserIsExistInGroup(&rcptInfo) {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"UserAddResponse",
			"Адресат с таким Email уже существует в группе.",
		}
		return
	}
	for _, rcptGroup := range GlobalGroups {
		if rcptInfo.GroupName == rcptGroup.GroupName {
			rcptGroup.GroupMails[rcptInfo.RcptEmail] = rcptInfo.RcptName
		}
	}
	if SaveRCPTInfoToDB(&rcptInfo) {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"UserAddResponse",
			"Адресат успешно добавлен.",
		}
		clientConn.AddressesGroupsRequest()
	} else {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"UserAddResponse",
			"Ошибка при добавлении адресата в БД.",
		}
	}
}

func (clientConn *ClientConn) UserRemoveRequest(message *ClientWSMessage) {
	var rcptInfo RcptInfo
	err := json.Unmarshal([]byte(message.MessageBody), &rcptInfo)
	if err != nil {
		Error.Println(err)
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"UserRemoveResponse",
			"Не удалось удалить адресата из-за ошибки парсинга JSON.",
		}
		return
	}
	if CheckUserIsExistInGroup(&rcptInfo) {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"UserRemoveResponse",
			"Адресата с таким Email нет в группе.",
		}
		return
	}
	for _, rcptGroup := range GlobalGroups {
		if rcptInfo.RcptEmail == rcptGroup.GroupName {
			delete(rcptGroup.GroupMails, rcptInfo.RcptEmail)
		}
	}
	if DeleteRCPTInfoToDB(&rcptInfo) {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"UserRemoveResponse",
			"Адресат успешно удален.",
		}
		clientConn.AddressesGroupsRequest()
	} else {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"UserRemoveResponse",
			"Ошибка при удалении адресата из БД.",
		}
	}

}

func CheckUserIsExistInGroup(rcptInfo *RcptInfo) bool {
	for _, rcptGroup := range GlobalGroups {
		if rcptInfo.GroupName == rcptGroup.GroupName {
			for email, _ := range rcptGroup.GroupMails {
				if rcptInfo.RcptEmail == email {
					return false
				}
			}
		}
	}
	return true
}

func CheckGroupIsExist(groupNameToFind string) bool {
	for _, groupName := range GlobalGroupNames {
		if groupName == groupNameToFind {
			return true
		}
	}
	return false
}

func (clientConn *ClientConn) AddressesGroupsRequest() {
	mailgroups, err := json.Marshal(&GlobalGroups)
	if err != nil {
		Error.Println("Cannot serialize global mail groups:", err)
		return
	}
	clientConn.ClientChannel <- &ClientWSMessage{
		GetDateString(),
		"AddressesGroupsResponse",
		string(mailgroups),
	}
}

func (clientConn *ClientConn) TemplateAddRequest(message *ClientWSMessage) {
	var template *MailTemplate
	err := json.Unmarshal([]byte(message.MessageBody), &template)
	if err != nil {
		Error.Println(err)
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"TemplateAddResponse",
			"Не удалось обработать темплейт из-за ошибки парсинга JSON.",
		}
		return
	}
	if CheckTemplateIsExist(template.TemplateName) {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"TemplateAddResponse",
			"Темплейт с таким именем уже существует.",
		}
		return
	}
	if SaveTemplateToDB(template) {
		WriteTemplateImages(template.TemplateName, template.TemplateImages)
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"TemplateAddResponse",
			"Темплейт успешно добавлен.",
		}
		clientConn.TemplatesRequest()
	} else {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"TemplateAddResponse",
			"Ошибка добавления темплейта в БД, смотреть логи.",
		}
	}
}

func CheckTemplateIsExist(templateNameToFind string) bool {
	for _, template := range GlobalTemplates {
		if templateNameToFind == template.TemplateName {
			return true
		}
	}
	return false
}

func (clientConn *ClientConn) TemplatesRequest() {
	if RefreshTemplatesFromDB() {
		templates, err := json.Marshal(GlobalTemplates)
		if err != nil {
			Error.Println(err)
			return
		}
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"TemplateRefresh",
			string(templates),
		}
	}
}

func (clientConn *ClientConn) TemplatesDeleteRequest(message *ClientWSMessage) {
	if !DeleteTemplateFromDB(message.MessageBody) {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"TemplateDeleteResponse",
			"Ошибка удаления темплейта из БД, смотреть логи.",
		}
	} else {
		clientConn.ClientChannel <- &ClientWSMessage{
			GetDateString(),
			"TemplateDeleteResponse",
			"Темплейт \"" + message.MessageBody + "\" успешно удалён.",
		}
		clientConn.TemplatesRequest()
	}
}

func GetDateString() string {
	t := time.Now()
	return t.Format("20060102150405")
}

func WriteTemplateImages(templateName string, templImages []*TemplateImage) {
	if len(templImages) == 0 {
		return
	}
	for _, templImage := range templImages {
		go WriteImageToStatic(templImage.ImageName, templImage.ImageBody, templateName)
	}
}

func WriteImageToStatic(imagename string, imageb64 string, templateName string) {
	if !StaticDirectory("templates") {
		Error.Println("Unable to create templates folder.")
		return
	}
	if !StaticDirectory("templates/" + templateName) {
		Error.Println("Unable to create template name folder.")
		return
	}
	var prefix = regexp.MustCompile(`.*image/`)
	var suffix = regexp.MustCompile(`;.*`)
	getExt := prefix.ReplaceAllString(imageb64, "")
	getExt2 := suffix.ReplaceAllString(getExt, "")
	if getExt2 == "" {
		Error.Println("Unknown mime type.")
	}
	imageAsBytes := DecodeFile(imageb64)
	filePath := "static/templates/" + templateName + "/" + imagename
	err := ioutil.WriteFile(filePath, imageAsBytes, 755)
	if err != nil {
		Error.Println(err)
	}
}

func DecodeFile(filestring string) []byte {
	data := filestring[strings.IndexByte(filestring, ',')+1:]
	dec, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		Error.Println(err)
		return nil
	}
	return dec
}

func getRouter() *mux.Router {
	router := mux.NewRouter()
	return router
}

func Configure() *Configuration {
	var config Configuration
	config.SetServicePort()
	config.SetDBParams()
	config.SetSMTPParams()
	defaultSmtpServer = config.SetSMTPParams()
	return &config
}

func (config *Configuration) SetServicePort() {
	sport := os.Getenv("M_SERVICE_PORT")
	if sport == "" {
		config.ServicePort = "8080"
	} else {
		config.ServicePort = sport
	}
}

func (config *Configuration) SetDBParams() {
	dbaddr := os.Getenv("M_DB_ADDR")
	if dbaddr == "" {
		//config.DBAddr = "127.0.0.1:5432"
		config.DBAddr = "192.168.1.210:5432"
	} else {
		config.DBAddr = dbaddr
	}
	dbuser := os.Getenv("M_DB_USER")
	if dbuser == "" {
		config.DBUser = "maileruser"
	} else {
		config.DBUser = dbuser
	}
	dbpass := os.Getenv("M_DB_PASS")
	if dbpass == "" {
		config.DBPass = "HRcnJ382x"
	} else {
		config.DBPass = dbpass
	}
	dbname := os.Getenv("M_DB_NAME")
	if dbname == "" {
		config.DBName = "mailer"
	} else {
		config.DBName = dbname
	}
}

func (config *Configuration) SetSMTPParams() *SmtpParams {
	var smtpparams SmtpParams
	server := os.Getenv("SMTP_SERVER")
	if server == "" {
		smtpparams.SMTPServer = "mail.itliga.ru"
	} else {
		smtpparams.SMTPServer = server
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		smtpparams.SMTPPort = "587"
	} else {
		smtpparams.SMTPPort = port
	}
	login := os.Getenv("SMTP_LOGIN")
	if login == "" {
		smtpparams.SMTPLogin = "management@avelamedia.ru"
	} else {
		smtpparams.SMTPLogin = login
	}
	pass := os.Getenv("SMTP_PASS")
	if pass == "" {
		smtpparams.SMTPPass = "Hsoq2648"
	} else {
		smtpparams.SMTPPass = pass
	}
	threads := os.Getenv("SMTP_THREADS")
	if threads == "" {
		smtpparams.Threads = "1"
	} else {
		smtpparams.Threads = threads
	}
	return &smtpparams
}

func (mailer *MailerApp) Load() {
	go GetRCPTGroupNamesFromDB()
	go RefreshTemplatesFromDB()
	mailer.Router.HandleFunc("/ws", mailer.wsHandler)
	mailer.Router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/")))
	httpServer := &http.Server{
		Handler:      mailer.Router,
		Addr:         "127.0.0.1:" + mailer.Conf.ServicePort,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
	if err := httpServer.ListenAndServe(); err != nil {
		Error.Println("Web Server Error:", err)
	}
}

func ServicesStart(config *Configuration) {
	mailer := MailerApp{
		config,
		getRouter(),
		DataBaseConnString(config),
		make([]*ClientConn, 0),
	}
	if TestDBConn(mailer.DBConnString) {
		DBConnectionString = mailer.DBConnString
		mailer.Load()
	} else {
		Error.Println("Unable to connect database", mailer.Conf.DBAddr)
	}
}

func init() {
	LogInit(ioutil.Discard, os.Stdout, os.Stdout, os.Stdout)
}

func main() {
	ServicesStart(Configure())
}
