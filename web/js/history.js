var ws;

function WSConnection() {
    if (location.protocol == 'https:'){
        ws = new WebSocket("wss://" + window.location.hostname + ":" + window.location.port + "/ws");
    } else {
        ws = new WebSocket("ws://" + window.location.hostname + ":" + window.location.port + "/ws");
    }

    ws.onopen = function(){
        ShowWSState("");
        RemoveAllHistory();
        RequestHistory();
    };

    ws.onmessage = function (event) {
        var message = JSON.parse(event.data);
        switch (message.messagetype) {
            case 'HistoryUpdateInstance':
                AddHistoryInstance(message.messagebody);
                break;
            case 'HistInstanceResponse':
                HistInstanceResponse(message.messagebody);
                break;
            default:
                alert("Внимание! От сервера пришла какая-то фигня.");
        }
    };

    ws.onclose = function(e){
        ShowWSState("Соединение с сервером разорвано. Ожидание подключения...");
        setTimeout(function() {
            WSConnection();
        }, 3000);
    };

    ws.onerror = function (err) {
        ShowWSState(err.message);
        ws.close();
    }
}

function ShowWSState(message) {
    var stateElem = document.getElementById("wsconnectioninfo");
    stateElem.innerText = message;
}

WSConnection();

var histForChildWindow;

function HistInstanceResponse(messagebody) {
    histForChildWindow = JSON.parse(messagebody);
    var messageField = document.getElementById("to-usermessage");
    messageField.innerText = "";
    myWindow = window.open(location.protocol + "//" + window.location.hostname + ":" + window.location.port,"_blank");
}

function RequestHistory() {
    RemoveAllHistory();
    if (ws.readyState == 1){
        var message = {
            "messagetype": "HistoryRequest", "messagebody": "Request"
        };
        ws.send(JSON.stringify(message));
    }
}

function RemoveAllHistory(){
    var parentElem = document.getElementById("history-body");
    while (parentElem.firstChild) {
        parentElem.removeChild(parentElem.firstChild);
    }
}

function AddHistoryInstance(messagebody){
    messageInfo = JSON.parse(messagebody);
    var parentElem = document.getElementById("history-body");
    parentElem.className = "historytable";

    var TRElem = document.createElement("tr");
    TRElem.id = messageInfo.MailDate;

    var THElem = document.createElement("th");
    THElem.scope = "row";
    var mdate = messageInfo.MailDate;
    THElem.innerText = mdate.slice(0,4) + "." + mdate.slice(4,6) + "." + mdate.slice(6,8) + " " + mdate.slice(8,10) + ":" + mdate.slice(10,12) + ":" + mdate.slice(12,14);
    THElem.className = "historytable";
    TRElem.appendChild(THElem);


    var senderMail = document.createElement("td");
    senderMail.innerText = messageInfo.SenderMail;
    senderMail.className = "historytable";
    TRElem.appendChild(senderMail);

    var MailTemplateName = document.createElement("td");
    MailTemplateName.innerText = messageInfo.MailTemplateName;
    MailTemplateName.className = "historytable";
    TRElem.appendChild(MailTemplateName);

    var MailTheme = document.createElement("td");
    MailTheme.innerText = messageInfo.MailTheme;
    MailTheme.className = "historytable";
    TRElem.appendChild(MailTheme);

    var MailAttach = document.createElement("td");
    MailAttach.className = "historytable";

    var IsConfirmed = document.createElement("td");
    IsConfirmed.innerText = messageInfo.IsConfirmed;
    if (messageInfo.IsConfirmed == false){
        IsConfirmed.innerText = "Нет";
    } else {
        IsConfirmed.innerText = "Да";
    }
    IsConfirmed.className = "historytable";
    TRElem.appendChild(IsConfirmed);

    var ConfirmMail = document.createElement("td");
    ConfirmMail.innerText = messageInfo.ConfirmMail;
    ConfirmMail.className = "historytable";
    TRElem.appendChild(ConfirmMail);

    var IsDelivered = document.createElement("td");
    if (messageInfo.IsDelivered == false){
        IsDelivered.innerText = "Не отправлено";
    } else {
        IsDelivered.innerText = "Отправлено";
    }
    IsDelivered.className = "historytable";
    TRElem.appendChild(IsDelivered);

    var RecipientGroups = document.createElement("td");
    RecipientGroups.innerText = messageInfo.RecipientGroups;
    RecipientGroups.className = "historytable";
    TRElem.appendChild(RecipientGroups);

    TRElem.onclick = function(){
        var messageField = document.getElementById("to-usermessage");
        messageField.innerText = "Инстанс загружается, подождите...";
        if (ws.readyState == 1){
            var message = {
                "messagetype": "HistoryInstanceRequest", "messagebody": TRElem.id
            };
            ws.send(JSON.stringify(message));
        }
    };
    parentElem.appendChild(TRElem);
}

function b64toBlob(b64Data) {
    if (b64Data == null || b64Data == ""){
        return
    }
    var contentType = base64MimeType(b64Data) || '';
    var sliceSize = b64Data.length || 512;

    var getPureBase64 = b64Data.split(',')[1];
    var byteCharacters = atob(getPureBase64);
    var byteArrays = [];

    for (var offset = 0; offset < byteCharacters.length; offset += sliceSize) {
        var slice = byteCharacters.slice(offset, offset + sliceSize);

        var byteNumbers = new Array(slice.length);
        for (var i = 0; i < slice.length; i++) {
            byteNumbers[i] = slice.charCodeAt(i);
        }

        var byteArray = new Uint8Array(byteNumbers);

        byteArrays.push(byteArray);
    }

    var blob = new Blob(byteArrays, {type: contentType});
    return blob;
}

function base64MimeType(encoded) {
    var result = null;

    if (typeof encoded !== 'string') {
        return result;
    }

    var mime = encoded.match(/data:([a-zA-Z0-9]+\/[a-zA-Z0-9-.+]+).*,.*/);

    if (mime && mime.length) {
        result = mime[1];
    }

    return result;
}