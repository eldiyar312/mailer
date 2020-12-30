package main

import (
	"database/sql"
	"encoding/json"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

const (
	SAVE_DELIVER_TO_DB           = "INSERT INTO public.history(\"MailDate\", \"SenderMail\", \"SenderName\", \"MailTemplateName\", \"MailTheme\", \"IsConfirmed\", \"ConfirmMail\", \"IsDelivered\", \"RecipientGroups\") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)"
	SAVE_ATTACHMENTS_TO_DB       = "INSERT INTO public.attachments(maildate, mailattachname, mailattach) VALUES ($1, $2, $3)"
	GET_RCPT_GROUPNAMES          = "SELECT DISTINCT groupname FROM public.rcptgroups"
	GET_RCPT_BY_GROUPNAME        = "SELECT email, rcptname FROM public.rcptgroups WHERE groupname = $1"
	SAVE_NEWSLETTERS_TO_DB       = "INSERT INTO public.newsletters(\"Header\", \"Annotation\", \"Link\", \"Image\", \"ImageLink\", \"MailDate\", \"NewsNumber\", \"NewsBlockName\", \"NewsBlockNumber\", \"NewsBlockLink\", \"Source\")	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)"
	GET_HISTORY_FROM_DB          = "SELECT * FROM public.history ORDER BY \"MailDate\" DESC"
	GET_HISTORY_INSTANCE_FROM_DB = "SELECT * FROM public.history WHERE \"MailDate\" = $1"
	GET_NEWSLETTERS_FROM_DB      = "SELECT \"Header\", \"Annotation\", \"Link\", \"Image\", \"NewsNumber\", \"NewsBlockName\", \"NewsBlockNumber\", \"NewsBlockLink\", \"ImageLink\", \"Source\" FROM public.newsletters WHERE \"MailDate\" = $1"
	ADD_NEW_GROUP_TO_DB          = "INSERT INTO public.rcptgroups(groupname, email, rcptname) VALUES ($1, $2, $3)"
	REMOVE_GROUP_FROM_DB         = "DELETE FROM public.rcptgroups WHERE groupname = $1"
	SAVE_USER_TO_DB              = "INSERT INTO public.rcptgroups(groupname, email, rcptname) VALUES ($1, $2, $3)"
	DELETE_USER_FROM_DB          = "DELETE FROM public.rcptgroups WHERE groupname = $1 AND email = $2"
	UPDATE_DELIVERY_STATE        = "UPDATE public.history SET \"IsDelivered\"= $1 WHERE \"MailDate\" = $2"
	SAVE_TEMPLATE_TO_DB          = "INSERT INTO public.templates(templatename, templateusername, templatebody) VALUES ($1, $2, $3)"
	REQUEST_TEMPLATES_FROM_DB    = "SELECT templatename, templateusername, templatebody FROM public.templates"
	DELETE_TEMPLATE_FROM_DB      = "DELETE FROM public.templates WHERE templateusername = $1"
	GET_TEMPLATE_FROM_DB         = "SELECT templatename, templatebody FROM public.templates WHERE templateusername = $1"
	GET_ATTACHMENTS_FROM_DB      = "SELECT mailattachname, mailattach FROM public.attachments WHERE maildate = $1"
)

func DataBaseConnString(config *Configuration) string {
	return "postgres://" + config.DBUser + ":" + config.DBPass + "@" + config.DBAddr + "/" + config.DBName + "?sslmode=disable"
}

func TestDBConn(connstring string) bool {
	db, err := sql.Open("postgres", connstring)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer db.Close()
	err = db.Ping()
	if err != nil {
		Error.Println(err)
		return false
	}
	return true
}

func GetRCPTGroupNamesFromDB() {
	var groupNames []string
	var groupname string
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return
	}
	defer db.Close()
	stmt, err := db.Prepare(GET_RCPT_GROUPNAMES)
	if err != nil {
		Error.Println(err)
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	if err != nil {
		Error.Println(err)
		return
	}
	for rows.Next() {
		err := rows.Scan(&groupname)
		if err != nil {
			Error.Println(err)
			return
		}
		groupNames = append(groupNames, groupname)
	}
	defer rows.Close()
	GlobalGroupNames = groupNames
	go GetRCPTGroupsFromDB()
}

func GetRCPTGroupsFromDB() {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return
	}
	defer db.Close()
	stmt, err := db.Prepare(GET_RCPT_BY_GROUPNAME)
	if err != nil {
		Error.Println(err)
		return
	}
	defer stmt.Close()
	GlobalGroups = GlobalGroups[:0]
	for _, groupname := range GlobalGroupNames {
		var (
			email    string
			rcptname string
		)
		rcptGroup := RcptGroup{
			groupname,
			make(map[string]string),
		}
		rows, err := stmt.Query(groupname)
		if err != nil {
			Error.Println(err)
			return
		}
		for rows.Next() {
			email = ""
			rcptname = ""
			rows.Scan(&email, &rcptname)
			if email != "" {
				rcptGroup.GroupMails[email] = rcptname
			}
		}
		GlobalGroups = append(GlobalGroups, rcptGroup)
		rows.Close()
	}
}

func SaveNewGroupNameToDB(groupName string) bool {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer db.Close()
	stmt, err := db.Prepare(ADD_NEW_GROUP_TO_DB)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer stmt.Close()
	if _, err := stmt.Exec(groupName, "", ""); err != nil {
		Error.Println(err)
		return false
	}
	GetRCPTGroupNamesFromDB()
	return true
}

func RemoveGroupNameFromDB(groupName string) bool {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer db.Close()
	stmt, err := db.Prepare(REMOVE_GROUP_FROM_DB)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer stmt.Close()
	if _, err := stmt.Exec(groupName); err != nil {
		Error.Println(err)
		return false
	}
	GetRCPTGroupNamesFromDB()
	return true
}

func SaveDeliverToDB(deliver *Deliver) bool {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer db.Close()
	stmt, err := db.Prepare(SAVE_DELIVER_TO_DB)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer stmt.Close()
	if _, err := stmt.Exec(
		deliver.MailDate,
		deliver.SenderMail,
		deliver.SenderName,
		deliver.MailTemplateName,
		deliver.MailTheme,
		deliver.IsConfirmed,
		deliver.ConfirmMail,
		deliver.IsDelivered,
		pq.Array(deliver.RecipientGroups),
	); err != nil {
		Error.Println(err)
		return false
	}
	stmt2, err := db.Prepare(SAVE_NEWSLETTERS_TO_DB)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer stmt2.Close()
	for _, newsBlock := range deliver.MailNews.NewsLettersBlocks {
		for _, news := range newsBlock.NewsLetters {
			if _, err := stmt2.Exec(
				news.Header,
				news.Annotation,
				news.Link,
				news.Image,
				news.ImageLink,
				deliver.MailDate,
				news.NewsNumber,
				newsBlock.BlockHeader,
				newsBlock.BlockNumber,
				newsBlock.BlockLink,
				news.Source,
			); err != nil {
				Error.Println(err)
			}
		}

	}
	stmt3, err := db.Prepare(SAVE_ATTACHMENTS_TO_DB)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer stmt3.Close()
	for _, attach := range deliver.Attachments {
		if _, err := stmt3.Exec(
			deliver.MailDate,
			attach.Name,
			attach.Body,
		); err != nil {
			Error.Println(err)
		}
	}
	return true
}

func UpdateDeliveryState(deliverId string, newstate bool) {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return
	}
	defer db.Close()
	stmt, err := db.Prepare(UPDATE_DELIVERY_STATE)
	if err != nil {
		Error.Println(err)
		return
	}
	defer stmt.Close()
	if _, err := stmt.Exec(newstate, deliverId); err != nil {
		Error.Println(err)
		return
	}
}

func GetHistoryFromDB(clientConn *ClientConn) {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return
	}
	defer db.Close()
	stmt, err := db.Prepare(GET_HISTORY_FROM_DB)
	if err != nil {
		Error.Println(err)
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	defer rows.Close()
	for rows.Next() {
		if !clientConn.IsAvailable {
			return
		}
		var deliver Deliver
		var (
			MailDate         string
			SenderMail       string
			SenderName       string
			MailTemplateName string
			MailTheme        string
			IsConfirmed      bool
			ConfirmMail      string
			IsDelivered      bool
			RecipientGroups  []string
		)
		err := rows.Scan(
			&MailDate,
			&SenderMail,
			&SenderName,
			&MailTemplateName,
			&MailTheme,
			&IsConfirmed,
			&ConfirmMail,
			&IsDelivered,
			pq.Array(&RecipientGroups),
		)
		if err != nil {
			Error.Println(err)
		}

		deliver.MailDate = MailDate
		deliver.SenderMail = SenderMail
		deliver.SenderName = SenderName
		deliver.MailTemplateName = MailTemplateName
		deliver.MailTheme = MailTheme
		deliver.IsConfirmed = IsConfirmed
		deliver.ConfirmMail = ConfirmMail
		deliver.IsDelivered = IsDelivered
		deliver.RecipientGroups = RecipientGroups
		deliver.MailNews = nil
		if clientConn.IsAvailable {
			histinstance, err := json.Marshal(&deliver)
			if err == nil {
				clientConn.ClientChannel <- &ClientWSMessage{
					"",
					"HistoryUpdateInstance",
					string(histinstance),
				}
			} else {
				Error.Println(err)
			}
		}
	}
}

func GetHistoryInstance(clientConn *ClientConn, deliverDate string) {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return
	}
	defer db.Close()
	stmt, err := db.Prepare(GET_HISTORY_INSTANCE_FROM_DB)
	if err != nil {
		Error.Println(err)
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query(deliverDate)
	defer rows.Close()
	for rows.Next() {
		if !clientConn.IsAvailable {
			return
		}
		var deliver Deliver
		var (
			MailDate         string
			SenderMail       string
			SenderName       string
			MailTemplateName string
			MailTheme        string
			IsConfirmed      bool
			ConfirmMail      string
			IsDelivered      bool
			RecipientGroups  []string
		)
		err := rows.Scan(
			&MailDate,
			&SenderMail,
			&SenderName,
			&MailTemplateName,
			&MailTheme,
			&IsConfirmed,
			&ConfirmMail,
			&IsDelivered,
			pq.Array(&RecipientGroups),
		)
		if err != nil {
			Error.Println(err)
		}

		deliver.MailDate = MailDate
		deliver.SenderMail = SenderMail
		deliver.SenderName = SenderName
		deliver.MailTemplateName = MailTemplateName
		deliver.MailTheme = MailTheme
		deliver.IsConfirmed = IsConfirmed
		deliver.ConfirmMail = ConfirmMail
		deliver.IsDelivered = IsDelivered
		deliver.RecipientGroups = RecipientGroups

		stmt2, err := db.Prepare(GET_NEWSLETTERS_FROM_DB)
		if err != nil {
			Error.Println(err)
		}
		defer stmt2.Close()

		var mailNewsLetters MailNewsLetters
		mailNewsLetters.NewsHeader = deliver.MailTheme
		mailNewsLetters.NewsLettersBlocks = []*NewsLettersBlock{}

		rows2, err := stmt2.Query(deliver.MailDate)
		defer rows2.Close()
		for rows2.Next() {
			var (
				header          string
				annotation      string
				link            string
				image           string
				newsnumber      string
				newsblockname   string
				newsblocknumber string
				newsblocklink   string
				imagelink       string
				source          string
			)
			err := rows2.Scan(
				&header,
				&annotation,
				&link,
				&image,
				&newsnumber,
				&newsblockname,
				&newsblocknumber,
				&newsblocklink,
				&imagelink,
				&source,
			)
			if err != nil {
				Error.Println(err)
			}

			var newsBlock NewsLettersBlock
			newsBlock.BlockHeader = newsblockname
			newsBlock.BlockNumber = newsblocknumber
			newsBlock.BlockLink = newsblocklink

			var newsLetter NewsLetter
			newsLetter.Header = header
			newsLetter.Annotation = annotation
			newsLetter.Link = link
			newsLetter.Image = image
			newsLetter.NewsNumber = newsnumber
			newsLetter.ImageLink = imagelink
			newsLetter.Source = source

			if BlockIsNotExist(&newsBlock, &mailNewsLetters) {
				newsBlock.NewsLetters = append(newsBlock.NewsLetters, &newsLetter)
				mailNewsLetters.NewsLettersBlocks = append(mailNewsLetters.NewsLettersBlocks, &newsBlock)
			} else {
				for _, block := range mailNewsLetters.NewsLettersBlocks {
					if newsBlock.BlockNumber == block.BlockNumber {
						block.NewsLetters = append(block.NewsLetters, &newsLetter)
					}
				}
			}
		}

		stmt3, err := db.Prepare(GET_ATTACHMENTS_FROM_DB)
		if err != nil {
			Error.Println(err)
		}
		defer stmt3.Close()

		rows3, err := stmt3.Query(deliver.MailDate)
		defer rows3.Close()
		for rows3.Next() {
			var (
				name string
				body string
			)
			err := rows3.Scan(
				&name,
				&body,
			)
			if err != nil {
				Error.Println(err)
			}
			var attachment Attachment
			attachment.Name = name
			attachment.Body = body
			deliver.Attachments = append(deliver.Attachments, &attachment)
		}

		deliver.MailNews = &mailNewsLetters

		if clientConn.IsAvailable {
			histinstance, err := json.Marshal(&deliver)
			if err == nil {
				clientConn.ClientChannel <- &ClientWSMessage{
					"",
					"HistInstanceResponse",
					string(histinstance),
				}
			} else {
				Error.Println(err)
			}
		}
	}
}

func BlockIsNotExist(newsBlock *NewsLettersBlock, mailNewsLetters *MailNewsLetters) bool {
	for _, block := range mailNewsLetters.NewsLettersBlocks {
		if newsBlock.BlockNumber == block.BlockNumber {
			return false
		}
	}
	return true
}

func SaveRCPTInfoToDB(rcptInfo *RcptInfo) bool {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer db.Close()
	stmt, err := db.Prepare(SAVE_USER_TO_DB)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer stmt.Close()
	if _, err := stmt.Exec(rcptInfo.GroupName, rcptInfo.RcptEmail, rcptInfo.RcptName); err != nil {
		Error.Println(err)
		return false
	}
	GetRCPTGroupNamesFromDB()
	return true
}

func DeleteRCPTInfoToDB(rcptInfo *RcptInfo) bool {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer db.Close()
	stmt, err := db.Prepare(DELETE_USER_FROM_DB)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer stmt.Close()
	if _, err := stmt.Exec(rcptInfo.GroupName, rcptInfo.RcptEmail); err != nil {
		Error.Println(err)
		return false
	}
	GetRCPTGroupNamesFromDB()
	return true
}

func SaveTemplateToDB(template *MailTemplate) bool {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer db.Close()
	stmt, err := db.Prepare(SAVE_TEMPLATE_TO_DB)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer stmt.Close()
	if _, err := stmt.Exec(template.TemplateFileName, template.TemplateName, template.TemplateFile); err != nil {
		Error.Println(err)
		return false
	}

	return true
}

func RefreshTemplatesFromDB() bool {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer db.Close()
	stmt, err := db.Prepare(REQUEST_TEMPLATES_FROM_DB)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer stmt.Close()
	GlobalTemplates = GlobalTemplates[:0]
	rows, err := stmt.Query()
	if err != nil {
		Error.Println(err)
		return false
	}
	for rows.Next() {
		var (
			templatename     string
			templateusername string
			templatebody     string
		)
		rows.Scan(&templatename, &templateusername, &templatebody)
		templ := MailTemplate{
			templatename,
			templatebody,
			templateusername,
			[]*TemplateImage{},
		}
		GlobalTemplates = append(GlobalTemplates, templ)
	}
	rows.Close()
	return true
}

func GetTemplateFromDB(templateName string) *MailTemplate {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return nil
	}
	defer db.Close()
	stmt, err := db.Prepare(GET_TEMPLATE_FROM_DB)
	if err != nil {
		Error.Println(err)
		return nil
	}
	defer stmt.Close()
	rows, err := stmt.Query(templateName)
	if err != nil {
		Error.Println(err)
		return nil
	}
	var templ MailTemplate
	templ.TemplateName = templateName
	templ.TemplateImages = []*TemplateImage{}
	for rows.Next() {
		var (
			templatename string
			templatebody string
		)
		rows.Scan(&templatename, &templatebody)
		templ.TemplateFileName = templatename
		templ.TemplateFile = templatebody
	}
	rows.Close()
	return &templ
}

func DeleteTemplateFromDB(templatename string) bool {
	db, err := sql.Open("postgres", DBConnectionString)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer db.Close()
	stmt, err := db.Prepare(DELETE_TEMPLATE_FROM_DB)
	if err != nil {
		Error.Println(err)
		return false
	}
	defer stmt.Close()
	if _, err := stmt.Exec(templatename); err != nil {
		Error.Println(err)
		return false
	}
	return true
}
