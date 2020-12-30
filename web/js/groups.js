var ws;

function WSConnection() {
    if (location.protocol == 'https:'){
        ws = new WebSocket("wss://" + window.location.hostname + ":" + window.location.port + "/ws");
    } else {
        ws = new WebSocket("ws://" + window.location.hostname + ":" + window.location.port + "/ws");
    }

    ws.onopen = function(){
        ShowWSState("");
        RecieveRCPTGroups();
    };

    ws.onmessage = function (event) {
        var message = JSON.parse(event.data);
        switch (message.messagetype) {
            case 'RCPTGroupsResponse':
                rcptGroups = message.messagebody.split(",");
                refreshRecepientGroup();
                break;
            case 'AddressesGroupsResponse':
                AddressesGroupsProcessing(message.messagebody);
                break;
            case 'GroupAddResponse':
                RecieveRCPTGroups();
                GroupAddAnswer(message.messagebody);
                break;
            case 'GroupRemoveResponse':
                RecieveRCPTGroups();
                GroupRemoveAnswer(message.messagebody);
                break;
            case 'UserAddResponse':
                AddUserResponse(message.messagebody);
                break;
            case 'UserRemoveResponse':
                RemoveUserResponse(message.messagebody);
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

var rcptGroups;
var rcptAddressesGroups = new Map();

function RecieveRCPTGroups() {
    if (ws.readyState == 1){
        var message = {
            "messagetype": "RCPTGroupsRequest", "messagebody": "DefaultUser"
        };
        ws.send(JSON.stringify(message))
    }
}

function GroupsAddressesRequest() {
    if (ws.readyState == 1){
        var message = {
            "messagetype": "AddressesGroupsRequest", "messagebody": ""
        };
        ws.send(JSON.stringify(message))
    }
}

function AddressesGroupsProcessing(messagebody) {
    var groups = JSON.parse(messagebody);
    Object.keys(groups).forEach(group => {
       var groupObj = groups[group];
       var gName = groupObj.GroupName;
       var gAddresses = groupObj.GroupMails;
       rcptAddressesGroups.set(gName, gAddresses);
     });
    ShowGroupAddresses();
}

var SelectedGroup;
var SelectedGroupName;

function ShowGroupAddresses() {
    var selectGroups = document.getElementById("recipient-select");

    for (var i = 0, l = selectGroups.options.length, o; i < l; i++){
        o = selectGroups.options[i];
        if ( o.selected == true ){
            SelectedGroup = rcptAddressesGroups.get(o.value);
            SelectedGroupName = o.value;
        }
    }

    var parentElem = document.getElementById("groupmails-info");
    while (parentElem.firstChild){
        parentElem.removeChild(parentElem.firstChild)
    }
    var recepientHigherDiv = document.createElement("div");
    recepientHigherDiv.className = "form-group";
    recepientHigherDiv.id = "emails-form";


    var recLabel = document.createElement("label");
    recLabel.for = "emails-select";

    var selectRcpGroups = document.createElement("select");
    selectRcpGroups.className = "form-control";
    selectRcpGroups.id = "emails-select";
    selectRcpGroups.style.width = "width:auto;";

    var rowsCount = 0;
     for (var mail in SelectedGroup){
        var selectOption = document.createElement("option");
        selectOption.innerHTML = mail + " (" + SelectedGroup[mail] + ")";
        rowsCount++;
        selectRcpGroups.appendChild(selectOption)
    }
    selectRcpGroups.size = rowsCount;

    recepientHigherDiv.appendChild(recLabel);
    recepientHigherDiv.appendChild(selectRcpGroups);
    parentElem.appendChild(recepientHigherDiv);
}

function refreshRecepientGroup() {
    var parentElem = document.getElementById("recipient-groups");
    while (parentElem.firstChild){
        parentElem.removeChild(parentElem.firstChild)
    }
    var recepientHigherDiv = document.createElement("div");
    recepientHigherDiv.className = "form-group";
    recepientHigherDiv.id = "recipients-form";


    var recLabel = document.createElement("label");
    recLabel.for = "recipient-select";

    var selectRcpGroups = document.createElement("select");
    selectRcpGroups.className = "form-control";
    selectRcpGroups.id = "recipient-select";
    selectRcpGroups.style.width = "width:auto;";
    selectRcpGroups.addEventListener("change", ShowGroupAddresses);

    var rowsCount = 0;
    for (var rgroup in rcptGroups){
        var selectOption = document.createElement("option");
        selectOption.innerHTML = rcptGroups[rgroup];
        rowsCount++;
        if (rowsCount == 1){
            selectOption.selected = true;
        }
        selectRcpGroups.appendChild(selectOption)
    }

    recepientHigherDiv.appendChild(recLabel);
    recepientHigherDiv.appendChild(selectRcpGroups);
    parentElem.appendChild(recepientHigherDiv);
    GroupsAddressesRequest();
}

function EditGroupsInterface() {
    var parentElem = document.getElementById("groupsedit-block");
    while (parentElem.firstChild){
        parentElem.removeChild(parentElem.firstChild)
    }
    var upperDiv = document.createElement("div");
    upperDiv.className = "input-group mb-3";

    var lowerDiv = document.createElement("div");
    lowerDiv.className = "input-group-prepend";

    var spanElem = document.createElement("span");
    spanElem.className = "input-group-text";
    spanElem.id = "group-to-add";
    spanElem.innerText = "Имя группы:";

    var editMail = document.createElement("input");
    editMail.type = "text";
    editMail.className = "form-control";
    editMail.setAttribute("aria-describedby", "user-group");
    editMail.setAttribute("aria-label", "Введите имя новой группы рассылки");
    editMail.placeholder = "Введите имя новой группы рассылки";
    editMail.id = "groupname-to-create";

    var saveButton = document.createElement("button");
    saveButton.className = "btn btn-success";
    saveButton.innerText = "Сохранить группу";
    saveButton.onclick = function () {
        var groupToCreate = document.getElementById("groupname-to-create");
        if (groupToCreate.value != "" || groupToCreate.value != null){
            if (ws.readyState == 1){
                var message = {
                    "messagetype": "GroupAddRequest", "messagebody": groupToCreate.value
                };
                ws.send(JSON.stringify(message));
                var parentElem = document.getElementById("groupsedit-block");
                while (parentElem.firstChild){
                    parentElem.removeChild(parentElem.firstChild);
                }
            }
        }
    };

    lowerDiv.appendChild(spanElem);
    upperDiv.appendChild(lowerDiv);
    upperDiv.appendChild(editMail);

    parentElem.appendChild(upperDiv);
    parentElem.appendChild(saveButton)
}

function GroupAddAnswer(message) {
    var groupAnswer = document.getElementById("groupeditresult");
    groupAnswer.innerText = message;
}

function GroupRemove() {
    var selectGroups = document.getElementById("recipient-select");

    var selectedGroupToRemove;
    for (var i = 0, l = selectGroups.options.length, o; i < l; i++){
        o = selectGroups.options[i];
        if ( o.selected == true ){
            selectedGroupToRemove = o.value;
        }
    }
    if (confirm('Вы точно хотите удалить группу \"' + selectedGroupToRemove + "\"?")){
        if (ws.readyState == 1){
            var message = {
                "messagetype": "GroupRemoveRequest", "messagebody": selectedGroupToRemove
            };
            ws.send(JSON.stringify(message))
        }
    } else {
        return
    }
}

function GroupRemoveAnswer(message) {
    var groupAnswer = document.getElementById("groupeditresult");
    groupAnswer.innerText = message;
}

function AddUserToGroup() {
    var userMailElem = document.getElementById("addedUserMail");
    var userNameElem = document.getElementById("addedUserName");
    if (userMailElem.value == "" || userNameElem.value == ""){
        alert('Нужно заполнить поля E-mail адрес и Имя адресата для добавления.');
        return;
    }
    if (checkEmail(userMailElem) == false){
        alert('Неправильный адрес E-mail.');
        userMailElem.focus;
        return;
    }
    if (SelectedGroupName == null || SelectedGroupName == ""){
        alert('Выберите группу для добавления адресата.');
        return;
    }
    if (ws.readyState == 1){
        var message = {
            "messagetype": "UserAddRequest", "messagebody": JSON.stringify({
                GroupName: SelectedGroupName,
                RcptEmail: userMailElem.value,
                RcptName: userNameElem.value,
            })
        };
        ws.send(JSON.stringify(message))
    }
}

function AddUserResponse(message){
    var userAnswer = document.getElementById("userEditsResponse");
    userAnswer.innerText = message;
    GroupsAddressesRequest();
}

function checkEmail(mail) {
    var filter = /^([a-zA-Z0-9_\.\-])+\@(([a-zA-Z0-9\-])+\.)+([a-zA-Z0-9]{2,4})+$/;
    if (!filter.test(mail.value)) {
        return false;
    }
    return true;
}

function RemoveUser() {
    var selectUser = document.getElementById("emails-select");
    var selectedMail = selectUser.options[selectUser.selectedIndex];
    if (selectedMail == null){
        alert("Для удаления адресата необходимо выделить его в списке.");
        selectedMail.focus;
        return;
    }
    var userMail = selectedMail.value.split(" ");
    if (ws.readyState == 1){
        var message = {
            "messagetype": "UserRemoveRequest", "messagebody": JSON.stringify({
                GroupName: SelectedGroupName,
                RcptEmail: userMail[0],
                RcptName: "",
            })
        };
        ws.send(JSON.stringify(message))
    }
}

function RemoveUserResponse(message) {
    var userAnswer = document.getElementById("userEditsResponse");
    userAnswer.innerText = message;
    GroupsAddressesRequest();
}