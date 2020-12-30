var ws;

function WSConnection() {
    if (location.protocol == 'https:'){
        ws = new WebSocket("wss://" + window.location.hostname + ":" + window.location.port + "/ws");
    } else {
        ws = new WebSocket("ws://" + window.location.hostname + ":" + window.location.port + "/ws");
    }

    ws.onopen = function(){
        ShowWSState("");
        GetTemplates();
    };

    ws.onmessage = function (event) {
        var message = JSON.parse(event.data);
        switch (message.messagetype) {
            case 'TemplateAddResponse':
                TemplateResponse(message.messagebody);
                refreshTemplates();
                break;
            case 'TemplateDeleteResponse':
                TemplateResponse(message.messagebody);
                refreshTemplates();
                break;
            case 'TemplateRefresh':
                templatesFromServer = JSON.parse(message.messagebody);
                refreshTemplates();
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

var templatesFromServer;
var selectedTemplate;

var templateAttachUploaded;
var templateAttachUploadedName;

var templateImages = [];

function GetTemplates() {
    if (ws.readyState == 1){
        var message = {
            "messagetype": "TemplatesRequest", "messagebody": ""
        };
        ws.send(JSON.stringify(message))
    }
}

function refreshTemplates() {
    if (templatesFromServer == null){
        return
    }

    var parentElem = document.getElementById("templates");

    while (parentElem.firstChild){
        parentElem.removeChild(parentElem.firstChild)
    }

    var recepientHigherDiv = document.createElement("div");
    recepientHigherDiv.className = "form-group";
    recepientHigherDiv.id = "templates-form";
    recepientHigherDiv.style.width = "50%";


    var recLabel = document.createElement("label");
    recLabel.for = "templates-select";

    var selectTemplates = document.createElement("select");
    selectTemplates.className = "form-control";
    selectTemplates.id = "templates-select";
    selectTemplates.style.width = "width:auto;";
    selectTemplates.addEventListener("change", SelectTemplate);

    var rowsCount = 0;
    templatesFromServer.forEach(function (template) {
        var selectOption = document.createElement("option");
        selectOption.innerHTML = template.TemplateName;
        rowsCount++;
        if (rowsCount == 1){
            selectOption.selected = true;
            selectedTemplate = template.TemplateName;
        }
        selectTemplates.appendChild(selectOption);
    });

    if (templatesFromServer.length == 1){
        selectedTemplate = templatesFromServer[0].TemplateName;
    }

    recepientHigherDiv.appendChild(recLabel);
    recepientHigherDiv.appendChild(selectTemplates);
    parentElem.appendChild(recepientHigherDiv);

}

function SelectTemplate() {
    var selectTemplate = document.getElementById("templates-select");
    for (var i = 0, l = selectTemplate.options.length, o; i < l; i++){
        o = selectTemplate.options[i];
        if ( o.selected == true ){
            selectedTemplate = o.value;
        }
    }
}

function RemoveTemplate() {
    if (selectedTemplate == null || selectedTemplate == ""){
        alert("Необходимо выделить темплейт в списке для удаления.");
        return;
    }
    if (ws.readyState == 1){
        var message = {
            "messagetype": "TemplateDeleteRequest", "messagebody": selectedTemplate
        };
        ws.send(JSON.stringify(message))
    }
}

function AddTemplate() {
    var parentElem = document.getElementById("templates-addform");

    while (parentElem.firstChild){
        parentElem.removeChild(parentElem.firstChild)
    }
    var uploadHigherDiv = document.createElement("div");
    uploadHigherDiv.className = "input-group";
    uploadHigherDiv.style.width = "50%";

    var upperDivTemplateName = document.createElement("div");
    upperDivTemplateName.className = "input-group mb-3";
    upperDivTemplateName.style.width = "50%";

    var templateNameInput = document.createElement("input");
    templateNameInput.type = "text";
    templateNameInput.className = "form-control";
    templateNameInput.setAttribute("aria-label", "Recipient's username");
    templateNameInput.setAttribute("aria-describedby", "basic-addon2");
    templateNameInput.placeholder = "Введите название...";
    templateNameInput.id = "template-name-input";
    templateNameInput.onchange = function() {
        ValidateTemplateName(templateNameInput);
    };
    upperDivTemplateName.appendChild(templateNameInput);

    var lowerDivTemplate = document.createElement("div");
    lowerDivTemplate.className = "input-group-append";

    var templateSpan = document.createElement("span");
    templateSpan.className = "input-group-text";
    templateSpan.id = "basic-addon2";
    templateSpan.innerText = ".template";
    lowerDivTemplate.appendChild(templateSpan);
    upperDivTemplateName.appendChild(lowerDivTemplate);

    var upperDiv = document.createElement("div");
    upperDiv.className = "input-group-prepend";

    var upperSpan = document.createElement("span");
    upperSpan.className = "input-group-text";
    upperSpan.id = "inputGroupFileAddon01";
    upperSpan.innerHTML = "Загрузить";
    upperDiv.appendChild(upperSpan);

    var lowerDiv = document.createElement("div");
    lowerDiv.className = "custom-file";

    var inputField = document.createElement("input");
    inputField.type = "file";
    inputField.className = "custom-file-input";
    inputField.id = "upload-template-file";
    inputField.setAttribute("aria-describedby", "inputGroupFileAddon01");
    inputField.accept = ".gohtml, .html";
    inputField.addEventListener('change', function (event) {
        var files = event.target.files;
        var reader = new FileReader();

        reader.onload = function (e) {
            document.getElementById("template-upload-label").innerHTML =  files[0].name;
            templateAttachUploaded = e.target.result;
        };

        templateAttachUploadedName = files[0].name;
        reader.readAsDataURL(files[0]);
    }, false);
    lowerDiv.appendChild(inputField);

    var lowerLabel = document.createElement("label");
    lowerLabel.className = "custom-file-label";
    lowerLabel.for = "upload-template-file";
    lowerLabel.innerText = "Выбрать темплейт";
    lowerLabel.id = "template-upload-label";
    lowerDiv.appendChild(lowerLabel);

    uploadHigherDiv.appendChild(upperDiv);
    uploadHigherDiv.appendChild(lowerDiv);

    var buttonDiv = document.createElement("div");
    buttonDiv.className = "upload-save-button";

    var uploadInfoDiv = document.createElement("div");
    uploadInfoDiv.id = "uploadInfo";
    uploadInfoDiv.className = "upload-save-button";

    var acceptButton = document.createElement("button");
    acceptButton.className = "btn btn-primary";
    acceptButton.innerText = "Сохранить";
    acceptButton.type = "button";
    acceptButton.onclick = function (){
        SaveTemplate();
    };
    buttonDiv.appendChild(acceptButton);

    var declineButton = document.createElement("button");
    declineButton.className = "btn btn-danger";
    declineButton.innerText = "Отменить";
    declineButton.type = "button";
    declineButton.style.marginLeft = "20px";
    declineButton.onclick = function (){
        ResetTemplate();
    };
    buttonDiv.appendChild(declineButton);

    var templateFilesUpperDiv = document.createElement("div");
    templateFilesUpperDiv.className = "input-group mb-3";
    templateFilesUpperDiv.style.marginTop = "20px";
    templateFilesUpperDiv.style.width = "50%";

    var templateFilesFirstDiv = document.createElement("div");
    templateFilesFirstDiv.className = "input-group-prepend";

    var templateFilesSpan = document.createElement("span");
    templateFilesSpan.className = "input-group-text";
    templateFilesSpan.id = "inputGroupFileAddon01";
    templateFilesSpan.innerText = "Файлы темплейта:";
    templateFilesFirstDiv.appendChild(templateFilesSpan);

    var templateFilesSecondDiv = document.createElement("div");
    templateFilesSecondDiv.className = "custom-file";


    var templateFilesUpload = document.createElement("input");
    templateFilesUpload.type = "file";
    templateFilesUpload.className = "custom-file-input";
    templateFilesUpload.id = "[]upload-template-images";
    templateFilesUpload.setAttribute("aria-describedby", "inputGroupFileAddon01");
    templateFilesUpload.multiple = true;
    templateFilesUpload.addEventListener('change', function (event) {
        templateImages = [];
        var fileList = [];
        var infoString = "Файлы для темплейта:\n";
        for (var i = 0; i < templateFilesUpload.files.length; i++){
            fileList.push(templateFilesUpload.files[i]);
            infoString = infoString + templateFilesUpload.files[i].name + "\n";
        }
        for (var a = 0; a < fileList.length; a++){
            var reader = new FileReader();

            reader.onload = function (fname) {
                return function (e){
                    var imageObj = {ImageName: fname, ImageBody: e.target.result};
                    templateImages.push(imageObj);
                };
            }(fileList[a].name);
            reader.readAsDataURL(fileList[a]);
        }

        var infoElem = document.getElementById("uploadInfo");
        infoElem.innerText = infoString;
    }, false);

    templateFilesSecondDiv.appendChild(templateFilesUpload);

    var templateFilesLabel = document.createElement("label");
    templateFilesLabel.className = "custom-file-label";
    templateFilesLabel.for = "upload-template-images";
    templateFilesLabel.id = "template-upload-images";
    templateFilesLabel.innerText = "Выберите файлы...";

    templateFilesUpperDiv.appendChild(templateFilesFirstDiv);
    templateFilesUpperDiv.appendChild(templateFilesSecondDiv);

    templateFilesSecondDiv.appendChild(templateFilesLabel);

    parentElem.appendChild(upperDivTemplateName);
    parentElem.appendChild(uploadHigherDiv);
    parentElem.appendChild(templateFilesUpperDiv);
    parentElem.appendChild(uploadInfoDiv);
    parentElem.appendChild(buttonDiv);
}

function SaveTemplate() {
    var parentElem = document.getElementById("templates-addform");

    var templateUserName = document.getElementById("template-name-input").value;

    if (templateAttachUploadedName == null || templateAttachUploaded == null || templateUserName == null || templateUserName === ""){
        alert("Для загрузки темплейта необходимо заполнить все поля.");
        return;
    }

    if (ValidateTemplateName(document.getElementById("template-name-input")) !== true){
        return;
    }

    if (ws.readyState == 1){
        var message = {
            "messagetype": "TemplateAddRequest", "messagebody": JSON.stringify({
                TemplateFileName: templateAttachUploadedName,
                TemplateFile: templateAttachUploaded,
                TemplateName: templateUserName,
                TemplateImages: templateImages,
            })
        };
        ws.send(JSON.stringify(message));
    }
    while (parentElem.firstChild){
        parentElem.removeChild(parentElem.firstChild)
    }
}

function ResetTemplate() {
    var parentElem = document.getElementById("templates-addform");
    while (parentElem.firstChild){
        parentElem.removeChild(parentElem.firstChild)
    }
}

function TemplateResponse(message) {
    var templateAnswer = document.getElementById("userTemplatesResponse");
    templateAnswer.innerText = message;
}

function ValidateTemplateName(templateNameInput) {
    var re = new RegExp("[a-zA-Z0-9]+$");
    if (re.test(templateNameInput.value) !== true){
        alert("Имя темплейта должно содержать только латинские буквы и цифры.");
        templateNameInput.focus();
        return false;
    }
    return true;
}