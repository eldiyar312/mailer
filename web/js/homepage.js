var debugMailer = false;
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
        GetTemplates();
    };

    ws.onmessage = function (event) {
        var message = JSON.parse(event.data);
        switch (message.messagetype){
            case 'RCPTGroupsResponse':
                rcptGroups = message.messagebody.split(",");
                refreshRecepientGroup();
                break;
            case "UnlockSaveButton":
                EnableButtonByID("save-button-elem");
                break;
            case "UnlockSendButton":
                EnableButtonByID("send-button-elem");
                break;
            case 'SaveDeliverState':
                ShowDeliverySaveState(message.messagebody);
                break;
            case 'TemplateRefresh':
                templatesFromServer = JSON.parse(message.messagebody);
                refreshTemplates();
                break;
            case 'SavedDeliverResponse':
                if (message.messagebody == "null"){
                    break;
                }
                var savedDeliver = JSON.parse(message.messagebody);
                SavedDeliverResponseProc(savedDeliver);
                break;
            case 'DeliverPreview':
                ShowDeliverySaveState("");
                EnableButtonByID("preview-button-elem");
                OpenDeliverPreview(message.messagebody);
                break;
            case 'ImageFromLinkResponseSuccess':
                var imageResponse = JSON.parse(message.messagebody);
                ImageLinkResponseProc(imageResponse);
                break;
            case 'ImageFromLinkResponseFail':
                console.log(message.messagebody);
                break;
            default:
                alert("Ошибка! От сервера пришла какая-то фигня.");
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

//Max attachment size to 10 mb
var maxAttachmentSize = 10485760;

var templatesFromServer;

var rcptGroups;

var mailAttach;
var mailAttachName;

var selectedTemplateName;

var NewsLettersBlocks = [];

var Attachments = [];

function OpenDeliverPreview(body){
    if (debugMailer === false){
        myWindow = window.open('about:blank',"_blank");
        myWindow.document.write(body);
        myWindow.focus();
        myWindow.document.close();
    } else {
        PrintBody(body)
    }
}

function PrintBody(body){
    console.log(body)
}

window.onload=function() {
    document.getElementById("sender").value = "DefaultSender";
    AddAttachmentsUploadForm();
    document.getElementById('newsletters-blocks').addEventListener('change', function (event) {
        if (event.target.files == null){
            return
        }
        var files = event.target.files;
        var ev = event.target.id;
        const reader = new FileReader();

        reader.onload = function (e) {
            var parentBlock = event.target.parentNode.parentNode.parentNode.parentNode;

            var blockId = parentBlock.getAttribute("NBlockId");
            var newsId = parentBlock.getAttribute("NNewsId");

            var newsAttach = e.target.result;
            NewsLettersBlocks.forEach(function (block) {
                if (block.BlockNumber.toString() === blockId){
                    block.NewsLetters.forEach(function (newsletter) {
                        if (newsletter.NewsNumber.toString() === newsId){
                            newsletter.ImageLink = "";
                            newsletter.ExternalLink = false;
                            newsletter.Image = newsAttach;
                            newsletter.ImageName = files[0].name;
                        }
                    });
                }
            });
            RefreshNewsLetters();
         };

        reader.readAsDataURL(files[0]);
    }, false);
    ParentWindowCheck();
};

function AddAttachmentsUploadForm() {
    var parentElem = document.getElementById("upload-attachments-area");

    var attachFilesUpperDiv = document.createElement("div");
    attachFilesUpperDiv.className = "input-group mb-3";
    attachFilesUpperDiv.style.width = "30%";

    var attachFilesFirstDiv = document.createElement("div");
    attachFilesFirstDiv.className = "input-group-prepend";

    var attachFilesSpan = document.createElement("span");
    attachFilesSpan.className = "input-group-text";
    attachFilesSpan.id = "inputGroupFileAddon01";
    attachFilesSpan.innerText = "Вложения:";
    attachFilesFirstDiv.appendChild(attachFilesSpan);

    var attachFilesSecondDiv = document.createElement("div");
    attachFilesSecondDiv.className = "custom-file";


    var attachFilesUpload = document.createElement("input");
    attachFilesUpload.type = "file";
    attachFilesUpload.className = "custom-file-input";
    attachFilesUpload.id = "[]upload-attachments";
    attachFilesUpload.setAttribute("aria-describedby", "inputGroupFileAddon01");
    attachFilesUpload.multiple = true;
    attachFilesUpload.addEventListener('change', function (event) {
        Attachments = [];
        var actualAttachmentSize = 0;
        var fileList = [];
        var infoString = "Добавлены файлы:\n";
        for (var i = 0; i < attachFilesUpload.files.length; i++){
            fileList.push(attachFilesUpload.files[i]);
            actualAttachmentSize = actualAttachmentSize + attachFilesUpload.files[i].size;
            infoString = infoString + attachFilesUpload.files[i].name + "\n";
        }
        for (var a = 0; a < fileList.length; a++){
            var reader = new FileReader();

            reader.onload = function (fname, fsize) {
                return function (e){
                    var imageObj = {Name: fname, Body: e.target.result};
                    Attachments.push(imageObj);
                };
            }(fileList[a].name, fileList[a].size);
            reader.readAsDataURL(fileList[a]);
        }
        var infoElem = document.getElementById("showAttachname");
        if (actualAttachmentSize >= maxAttachmentSize){
            Attachments = [];
            alert("Общий размер вложений не должен превышать 10 мегабайт. На данный момент размер " + bytesToSize(actualAttachmentSize) + ".")
        } else {
            infoElem.innerText = infoString + "\nОбщий размер вложений: " + bytesToSize(actualAttachmentSize);
        }
    }, false);

    attachFilesSecondDiv.appendChild(attachFilesUpload);

    var attachFilesLabel = document.createElement("label");
    attachFilesLabel.className = "custom-file-label";
    attachFilesLabel.for = "upload-attachments";
    attachFilesLabel.id = "attachments-upload-files";
    attachFilesLabel.innerText = "Выберите файлы...";

    attachFilesUpperDiv.appendChild(attachFilesFirstDiv);
    attachFilesUpperDiv.appendChild(attachFilesSecondDiv);

    attachFilesSecondDiv.appendChild(attachFilesLabel);

    parentElem.appendChild(attachFilesUpperDiv);

    var removeFilesButton = document.createElement("button");
    removeFilesButton.type = "button";
    removeFilesButton.className = "btn btn-danger";
    removeFilesButton.innerText = "Удалить файлы";
    removeFilesButton.style.marginLeft = "10px";

    removeFilesButton.onclick = function () {
        RemoveUploadedFiles();
    };

    attachFilesUpperDiv.appendChild(removeFilesButton)
}

function RemoveUploadedFiles() {
    Attachments = [];
    var parentElem = document.getElementById("showAttachname");
    while (parentElem.firstChild) {
        parentElem.removeChild(parentElem.firstChild);
    }
}

function bytesToSize(bytes) {
    var sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    if (bytes == 0) return 'n/a';
    var i = parseInt(Math.floor(Math.log(bytes) / Math.log(1024)));
    if (i == 0) return bytes + ' ' + sizes[i];
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + sizes[i];
}

function SavedDeliverRequest() {
    if (ws.readyState == 1){
        var message = {
            "messagetype": "SavedDeliverRequest", "messagebody": "DefaultUser"
        };
        ws.send(JSON.stringify(message))
    }
}

function ClearDeliverRequest() {
    Attachments = [];
    document.getElementById("theme").value = null;
    mailAttach = null;
    mailAttachName = null;
    document.getElementById('showAttachname').innerHTML = null;
    NewsLettersBlocks = [];
    RefreshNewsBlocks();
    RefreshNewsLetters();
    selectedTemplateName = null;
}

function ParentWindowCheck() {
    if (window.opener != null){
        GetParentWindowData(window.opener.histForChildWindow);
    }
}

function GetParentWindowData(savedDeliver) {
    document.getElementById("sender").value = savedDeliver.SenderName;
    document.getElementById("theme").value = savedDeliver.MailTheme;
    if (savedDeliver.Attachments !== null){
            savedDeliver.Attachments.forEach(function (attach){
                Attachments.push(attach);
            });

        var actualAttachmentSize = 0;
        var infoString = "Добавлены файлы:\n";

        Attachments.forEach(function (attach){
            actualAttachmentSize = actualAttachmentSize + attach.Body.length;
            infoString = infoString + attach.Name + "\n";
        });
        var infoElem = document.getElementById("showAttachname");
        if (actualAttachmentSize >= maxAttachmentSize){
            Attachments = [];
            alert("Общий размер вложений не должен превышать 10 мегабайт. На данный момент размер " + bytesToSize(actualAttachmentSize) + ".")
        } else {
            infoElem.innerText = infoString + "\nОбщий размер вложений: " + bytesToSize(actualAttachmentSize);
        }
    }

    if (savedDeliver.MailNews !== null){
        savedDeliver.MailNews.NewsLettersBlocks.forEach(function(block){
            block.NewsLetters.forEach(function (newsletter) {
                var nnumber = parseInt(newsletter.NewsNumber, 10);
                newsletter.NewsNumber = nnumber;
                if (newsletter.ImageLink == ""){
                    newsletter.ExternalLink = false;
                } else {
                    newsletter.ExternalLink = true;
                }
            });
            var bnumber = parseInt(block.BlockNumber, 10);
            block.BlockNumber = bnumber;
        });
        NewsLettersBlocks = savedDeliver.MailNews.NewsLettersBlocks;
        RefreshNewsBlocks();
        RefreshNewsLetters();
    } else {
        NewsLettersBlocks = [];
        RefreshNewsBlocks();
        RefreshNewsLetters();
    }
    selectedTemplateName = savedDeliver.MailTemplateName;
}

function RecieveRCPTGroups() {
    if (ws.readyState == 1){
        var message = {
            "messagetype": "RCPTGroupsRequest", "messagebody": "DefaultUser"
        };
        ws.send(JSON.stringify(message))
    }
}

function NewsBlock(blocknumber) {
    this.BlockHeader = "";
    this.BlockNumber = blocknumber;
    this.BlockLink = "";
    this.NewsLetters = [];
}

function AddNewsBlock() {
    if (NewsLettersBlocks.length >= 5){
        alert("Новостных блоков не может быть более 5.");
        return;
    }
    var newsBlockCountArray = [];
    var newsBlockCount = 1;

    for ( var i = 0; i < NewsLettersBlocks.length; i++){
        newsBlockCountArray.push(NewsLettersBlocks[i].BlockNumber);
    }

    for ( var a = 1; i < 6; a++){
        if (newsBlockCountArray.includes(a) === false){
            newsBlockCount = a;
            break;
        }
    }
    var newsBlock = new NewsBlock(newsBlockCount);
    NewsLettersBlocks.push(newsBlock);
    RefreshNewsBlocks();
}

function RefreshNewsBlocks() {
    NewsLettersBlocks.sort(function (a, b) {
        if (a.BlockNumber > b.BlockNumber) {
            return 1;
        }
        if (a.BlockNumber < b.BlockNumber) {
            return -1;
        }
        return 0;
    });
    var parentElem = document.getElementById("newsletters-blocks");
    while (parentElem.firstChild){
        parentElem.removeChild(parentElem.firstChild);
    }
    NewsLettersBlocks.forEach(function (block) {
        var parentElem = document.getElementById("newsletters-blocks");

        var upperDiv = document.createElement("div");
        upperDiv.className = "input-group mb-3";
        upperDiv.id = "inputheader" + block.BlockNumber.toString();
        upperDiv.style.width = "60%";
        upperDiv.style.marginLeft = "1%";

        var lowerDiv = document.createElement("div");
        lowerDiv.className = "input-group-prepend";
        var span = document.createElement("span");
        span.className = "input-group-text";
        span.id = "newblockadd" + block.BlockNumber.toString();
        span.innerText = "Заголовок блока:";
        lowerDiv.appendChild(span);
        upperDiv.appendChild(lowerDiv);

        var inputNewsBlockName = document.createElement("input");
        inputNewsBlockName.type = "text";
        inputNewsBlockName.className = "form-control";
        inputNewsBlockName.required = true;
        if (block.BlockHeader === ""){
            inputNewsBlockName.placeholder = "";
        } else {
            inputNewsBlockName.value = block.BlockHeader;
        }
        inputNewsBlockName.setAttribute("aria-label", "NewsBlock");
        inputNewsBlockName.setAttribute("aria-describedby", "newblockadd" + block.BlockNumber.toString());
        inputNewsBlockName.onchange = function(){
            var blockId = inputNewsBlockName.parentNode.parentNode.getAttribute("newsblockId");
            NewsLettersBlocks.forEach(function (block) {
                if (block.BlockNumber.toString() === blockId){
                    block.BlockHeader = inputNewsBlockName.value;
                }
            });
            inputNewsBlockName.innerText = inputNewsBlockName.value;
        };
        upperDiv.appendChild(inputNewsBlockName);

        var upperDivLink = document.createElement("div");
        upperDivLink.className = "input-group mb-3";
        upperDivLink.id = "inputlink" + block.BlockNumber.toString();
        upperDivLink.style.width = "60%";
        upperDivLink.style.marginLeft = "1%";

        var lowerDivLink = document.createElement("div");
        lowerDivLink.className = "input-group-prepend";
        var spanLink = document.createElement("span");
        spanLink.className = "input-group-text";
        spanLink.id = "newblocklink" + block.BlockNumber.toString();
        spanLink.innerText = "Ссылка:";
        lowerDivLink.appendChild(spanLink);
        upperDivLink.appendChild(lowerDivLink);

        var inputNewsBlockLink = document.createElement("input");
        inputNewsBlockLink.type = "text";
        inputNewsBlockLink.className = "form-control";
        inputNewsBlockLink.required = true;
        if (block.BlockLink === ""){
            inputNewsBlockLink.placeholder = "";
        } else {
            inputNewsBlockLink.value = block.BlockLink;
        }
        inputNewsBlockLink.setAttribute("aria-label", "NewsBlock");
        inputNewsBlockLink.setAttribute("aria-describedby", "newblockadd" + block.BlockNumber.toString());
        inputNewsBlockLink.onchange = function(){
            var blockId = inputNewsBlockLink.parentNode.parentNode.getAttribute("newsblockId");
            NewsLettersBlocks.forEach(function (block) {
                if (block.BlockNumber.toString() === blockId){
                    block.BlockLink = inputNewsBlockLink.value;
                }
            });
            inputNewsBlockLink.innerText = inputNewsBlockLink.value;
        };
        upperDivLink.appendChild(inputNewsBlockLink);

        var divForNewsletters = document.createElement("div");
        divForNewsletters.className = "newsletters-forms";
        divForNewsletters.id = "newsletters" + block.BlockNumber.toString();

        var newsBlockDiv = document.createElement("div");
        newsBlockDiv.id = "blockid" + block.BlockNumber.toString();
        newsBlockDiv.className = "newsblock-class";
        newsBlockDiv.setAttribute("newsblockId", block.BlockNumber.toString());

        var removeBlockButton = document.createElement("button");
        removeBlockButton.type = "button";
        removeBlockButton.className = "btn btn-danger";
        removeBlockButton.innerText = "Удалить этот блок";
        removeBlockButton.style.marginLeft = "10px";

        var addNewsButton = document.createElement("button");
        addNewsButton.type = "button";
        addNewsButton.className = "btn btn-primary";
        addNewsButton.innerText = "Добавить новость в блок " + block.BlockNumber.toString();
        addNewsButton.style.marginTop = "10px";
        addNewsButton.style.marginLeft = "10px";
        addNewsButton.id = "newsbutton" + block.BlockNumber.toString();

        var blockPhar = document.createElement("p");
        newsBlockDiv.appendChild(blockPhar);

        var blockNumberPub = document.createElement("h3");
        blockNumberPub.innerText = "Блок номер " + block.BlockNumber.toString();
        newsBlockDiv.appendChild(blockNumberPub);
        newsBlockDiv.appendChild(blockPhar);

        newsBlockDiv.appendChild(upperDiv);
        newsBlockDiv.appendChild(upperDivLink);
        newsBlockDiv.appendChild(removeBlockButton);


        newsBlockDiv.appendChild(divForNewsletters);
        newsBlockDiv.appendChild(addNewsButton);
        newsBlockDiv.appendChild(blockPhar);

        parentElem.appendChild(blockPhar);
        parentElem.appendChild(newsBlockDiv);

        removeBlockButton.onclick = function () {
            RemoveNewsBlock(removeBlockButton);
        };
        addNewsButton.onclick = function () {
            AddNewsLetter(addNewsButton);
        };
    });
    RefreshNewsLetters();
}

function RemoveNewsBlock(removeBlockButton) {
    var blockId = removeBlockButton.parentNode.getAttribute("newsblockId");
    for ( var i = 0; i < NewsLettersBlocks.length; i++){
        if (NewsLettersBlocks[i].BlockNumber.toString() === blockId){
            NewsLettersBlocks.splice(i, 1)
        }
    }
    RefreshNewsBlocks();
}

function NewsLetter(number) {
    this.NewsNumber = number;
    this.Header = "";
    this.Annotation = "";
    this.Link = "";
    this.Image = "";
    this.ImageName = "";
    this.ImageLink = "";
    this.ExternalLink = true;
    this.Source = "";
}

function AddNewsLetter(addNewsButton) {
    var blockId = addNewsButton.parentNode.getAttribute("newsblockId");
    NewsLettersBlocks.forEach(function (block) {
       if (block.BlockNumber.toString() === blockId){
           block.NewsLetters.sort(function (a, b) {
               if (a.NewsNumber > b.NewsNumber) {
                   return 1;
               }
               if (a.NewsNumber < b.NewsNumber) {
                   return -1;
               }
               return 0;
           });
           var newsarray = [];
           block.NewsLetters.forEach(function (newsletter) {
               newsarray.push(parseInt(newsletter.NewsNumber, 10))
           });
           var newsNum = 1;
           if (newsarray.length < 1){
               newsNum = 1;
           } else {
               for(var i = 1; i <= newsarray.length; i++) {
                   if(newsarray.indexOf(i + 1) == -1){
                       newsNum = i + 1;
                   }
               }
           }
           var newsLetter = new NewsLetter(newsNum);
           block.NewsLetters.push(newsLetter);
       }
    });
    RefreshNewsLetters();
}

function RefreshNewsLetters() {
    NewsLettersBlocks.forEach(function (block) {
        block.NewsLetters.sort(function (a, b) {
            if (a.NewsNumber > b.NewsNumber) {
                return 1;
            }
            if (a.NewsNumber < b.NewsNumber) {
                return -1;
            }
            return 0;
        })
    });

    NewsLettersBlocks.forEach(function (block){
        var parentElem = document.getElementById("newsletters" + block.BlockNumber);
        if (parentElem !== null && parentElem.firstChild){
            while (parentElem.firstChild){
                parentElem.removeChild(parentElem.firstChild);
            }
        }
    });

    NewsLettersBlocks.forEach(function (block){
        block.NewsLetters.forEach(function (newsletter) {
            var parentElem = document.getElementById("newsletters" + block.BlockNumber);

            var upperDiv = document.createElement("div");
            upperDiv.id = "newsfield" + newsletter.NewsNumber;
            upperDiv.setAttribute("NBlockId", block.BlockNumber);
            upperDiv.setAttribute("NNewsId", newsletter.NewsNumber);
            upperDiv.style.padding = "10px";


            var newsLegend = document.createElement("h4");
            newsLegend.innerHTML = "Новость " + newsletter.NewsNumber;

            var removeButton = document.createElement("button");
            removeButton.className = "btn btn-danger btn-sm";
            removeButton.innerText = "Удалить новость";
            removeButton.style.marginLeft = "20px";
            removeButton.onclick = function(){
                var blockId = newsLink.parentNode.parentNode.getAttribute("NBlockId");
                var newsId = newsLink.parentNode.parentNode.getAttribute("NNewsId");
                NewsLettersBlocks.forEach(function (block) {
                    if (block.BlockNumber.toString() === blockId){
                        for ( var i = 0; i < block.NewsLetters.length; i++){
                            if (block.NewsLetters[i].NewsNumber.toString() === newsId){
                                block.NewsLetters.splice(i, 1)
                            }
                        }
                    }
                });
                RefreshNewsLetters();
            };

            newsLegend.appendChild(removeButton);

            var newsMoveUp = document.createElement("button");
            newsMoveUp.className = "btn btn-info btn-sm";
            newsMoveUp.innerText = "Вверх";
            newsMoveUp.style.marginLeft = "20%";
            newsMoveUp.style.width = "60px";
            newsMoveUp.onclick = function(){
                var blockId = newsLink.parentNode.parentNode.getAttribute("NBlockId");
                var newsId = newsLink.parentNode.parentNode.getAttribute("NNewsId");
                NewsLettersBlocks.forEach(function (block) {
                    if (block.BlockNumber.toString() === blockId){
                        for ( var i = 0; i < block.NewsLetters.length; i++){
                            if (block.NewsLetters[i].NewsNumber.toString() === newsId){
                                if (newsId <= 1){
                                    return
                                }
                                var acNews = block.NewsLetters[i].NewsNumber;
                                block.NewsLetters[i-1].NewsNumber = acNews;
                                block.NewsLetters[i].NewsNumber = acNews - 1;
                                break;
                            }
                        }
                    }
                });
                RefreshNewsLetters();
            };

            newsLegend.appendChild(newsMoveUp);

            var newsMoveDown = document.createElement("button");
            newsMoveDown.className = "btn btn-info btn-sm";
            newsMoveDown.innerText = "Вниз";
            newsMoveDown.style.marginLeft = "1%";
            newsMoveDown.style.width = "60px";
            newsMoveDown.onclick = function(){
                var blockId = newsLink.parentNode.parentNode.getAttribute("NBlockId");
                var newsId = newsLink.parentNode.parentNode.getAttribute("NNewsId");
                NewsLettersBlocks.forEach(function (block) {
                    if (block.BlockNumber.toString() === blockId){
                        for ( var i = 0; i < block.NewsLetters.length; i++){
                            if (block.NewsLetters[i].NewsNumber.toString() === newsId){
                                if (block.NewsLetters[i+1] == null){
                                    return
                                }
                                var acNews = block.NewsLetters[i].NewsNumber;
                                var nextNews = block.NewsLetters[i+1].NewsNumber;
                                if ((nextNews - 1) < 1){
                                    return
                                }
                                block.NewsLetters[i+1].NewsNumber = nextNews - 1;
                                block.NewsLetters[i].NewsNumber = acNews + 1;
                                break;
                            }
                        }
                    }
                });
                RefreshNewsLetters();
            };

            newsLegend.appendChild(newsMoveDown);

            var headerParagraph = document.createElement("p");
            headerParagraph.className = "headerParagraph";

            var sourceParagraph = document.createElement("p");
            sourceParagraph.className = "sourceParagraph";

            var annotationParagraph = document.createElement("p");
            annotationParagraph.className = "annotationParagraph";

            var linkParagraph = document.createElement("p");
            linkParagraph.className = "linkParagraph";

            var fileParagraph = document.createElement("p");
            fileParagraph.className = "fileParagraph";

            var newsHeader = document.createElement("input");
            newsHeader.type = "text";
            newsHeader.id = "newsHeader" + newsletter.NewsNumber;
            newsHeader.className = "form-control short-fields";
            newsHeader.required = true;
            if (newsletter.Header === ""){
                newsHeader.placeholder = "Заголовок";
            } else {
                newsHeader.value = newsletter.Header;
            }
            newsHeader.onchange = function () {
                var blockId = newsHeader.parentNode.parentNode.getAttribute("NBlockId");
                var newsId = newsHeader.parentNode.parentNode.getAttribute("NNewsId");
                NewsLettersBlocks.forEach(function (block) {
                    if (block.BlockNumber.toString() === blockId){
                        block.NewsLetters.forEach(function (newsletter) {
                           if (newsletter.NewsNumber.toString() === newsId){
                               newsletter.Header = newsHeader.value;
                           }
                        });
                    }
                });
                newsHeader.innerText = newsHeader.value;
            };

            var newsSource = document.createElement("input");
            newsSource.type = "text";
            newsSource.id = "newsSource" + newsletter.NewsNumber;
            newsSource.className = "form-control short-fields";
            newsSource.required = false;
            if (newsletter.Source === ""){
                newsSource.placeholder = "Источник";
            } else {
                newsSource.value = newsletter.Source;
                console.log(newsletter);
            }
            newsSource.onchange = function () {
                var blockId = newsSource.parentNode.parentNode.getAttribute("NBlockId");
                var newsId = newsSource.parentNode.parentNode.getAttribute("NNewsId");
                NewsLettersBlocks.forEach(function (block) {
                    if (block.BlockNumber.toString() === blockId){
                        block.NewsLetters.forEach(function (newsletter) {
                            if (newsletter.NewsNumber.toString() === newsId){
                                newsletter.Source = newsSource.value;
                            }
                        });
                    }
                });
                newsSource.innerText = newsSource.value;
            };

            var newsAnnotation = document.createElement("textarea");
            newsAnnotation.id = "newsAnnotation" + newsletter.NewsNumber;
            newsAnnotation.className = "form-control";
            newsAnnotation.required = true;
            if (newsletter.Annotation === ""){
                newsAnnotation.placeholder = "Аннотация";
            } else {
                newsAnnotation.value = newsletter.Annotation;
            }
            newsAnnotation.style.height = "20%";
            newsAnnotation.onchange = function () {
                var blockId = newsAnnotation.parentNode.parentNode.getAttribute("NBlockId");
                var newsId = newsAnnotation.parentNode.parentNode.getAttribute("NNewsId");
                NewsLettersBlocks.forEach(function (block) {
                    if (block.BlockNumber.toString() === blockId){
                        block.NewsLetters.forEach(function (newsletter) {
                            if (newsletter.NewsNumber.toString() === newsId){
                                newsletter.Annotation = newsAnnotation.value;
                            }
                        });
                    }
                });
                newsAnnotation.innerText = newsAnnotation.value;
            };

            var newsLink = document.createElement("input");
            newsLink.type = "text";
            newsLink.id = "newsLink" + newsletter.NewsNumber;
            newsLink.className = "form-control short-fields";
            newsLink.required = true;
            if (newsletter.Link === ""){
                newsLink.placeholder = "Ссылка на новость в формате http://example.com";
            } else {
                newsLink.value = newsletter.Link;
            }
            newsLink.onchange = function () {
                var blockId = newsLink.parentNode.parentNode.getAttribute("NBlockId");
                var newsId = newsLink.parentNode.parentNode.getAttribute("NNewsId");
                NewsLettersBlocks.forEach(function (block) {
                    if (block.BlockNumber.toString() === blockId){
                        block.NewsLetters.forEach(function (newsletter) {
                            if (newsletter.NewsNumber.toString() === newsId){
                                newsletter.Link = newsLink.value;
                            }
                        });
                    }
                });
                newsLink.innerText = newsLink.value;
            };

            var newsImageHigherDiv = document.createElement("div");
            newsImageHigherDiv.className = "input-group mb-3";

            var newsImageUpperDiv = document.createElement("div");
            newsImageUpperDiv.className = "input-group-prepend";

            var newsImageUpperSpan = document.createElement("span");
            newsImageUpperSpan.className = "input-group-text";
            newsImageUpperSpan.innerHTML = "Загрузить:";

            var newsImageLowerDiv = document.createElement("div");
            newsImageLowerDiv.className = "custom-file";

            headerParagraph.appendChild(newsHeader);
            annotationParagraph.appendChild(newsAnnotation);
            sourceParagraph.appendChild(newsSource);
            linkParagraph.appendChild(newsLink);

            AddedLinkOrFileSwitch(fileParagraph, block.BlockNumber, newsletter.NewsNumber);


            var pharFormat = document.createElement("p");

            newsImageUpperDiv.appendChild(newsImageUpperSpan);

            if (newsletter.ExternalLink === true){
                var addedInputsLink = AddFileExternalLink(block, newsletter);
                newsImageLowerDiv.appendChild(addedInputsLink);
            } else {
                var addedInputsFile = AddFileFromLocalInput(newsletter);
                newsImageLowerDiv.appendChild(addedInputsFile[0]);
                newsImageLowerDiv.appendChild(addedInputsFile[1]);
            }


            var linkImageDiv = document.createElement("div");
            linkImageDiv.className = "attached-images-preview";

            if (newsletter.Image !== "" && newsletter.Image !== undefined){
                var image = new Image();
                image.src = newsletter.Image;
                image.style.width = "160px";
                image.style.height = "120px";
                linkImageDiv.appendChild(image);
            }

            newsImageHigherDiv.appendChild(newsImageUpperDiv);
            newsImageHigherDiv.appendChild(newsImageLowerDiv);
            fileParagraph.appendChild(newsImageHigherDiv);

            upperDiv.appendChild(newsLegend);
            upperDiv.appendChild(headerParagraph);
            upperDiv.appendChild(sourceParagraph);
            upperDiv.appendChild(annotationParagraph);
            upperDiv.appendChild(linkParagraph);
            upperDiv.appendChild(fileParagraph);
            upperDiv.appendChild(linkImageDiv);

            parentElem.appendChild(pharFormat);
            parentElem.appendChild(upperDiv);
        });
    });
}

function AddedLinkOrFileSwitch(fileParagraph, blocknumber, newsletternumber) {
    var isExternal = true;
    NewsLettersBlocks.forEach(function (block) {
        if (block.BlockNumber == blocknumber){
            block.NewsLetters.forEach(function (newsletter) {
                if (newsletter.NewsNumber == newsletternumber){
                    if (newsletter.ExternalLink == false){
                        isExternal = false
                    }
                }
            });
        }
    });

    var switchLinkUpperDiv = document.createElement("div");
    switchLinkUpperDiv.className = "form-check form-check-inline";
    switchLinkUpperDiv.style.paddingBottom = "20px";
    switchLinkUpperDiv.style.marginLeft = "1%";

    var switchLinkInput = document.createElement("input");
    switchLinkInput.className = "form-check-input";
    switchLinkInput.type = "checkbox";
    switchLinkInput.id = "fileswitch1";
    switchLinkInput.value = "link";
    switchLinkInput.checked = isExternal;
    switchLinkUpperDiv.appendChild(switchLinkInput);

    var switchLinkLabel = document.createElement("label");
    switchLinkLabel.className = "form-check-label";
    switchLinkLabel.for = "fileswitch1";
    switchLinkLabel.innerText = "Ссылка";
    switchLinkUpperDiv.appendChild(switchLinkLabel);

    var switchFileUpperDiv = document.createElement("div");
    switchFileUpperDiv.className = "form-check form-check-inline";
    switchFileUpperDiv.style.paddingBottom = "20px";

    var switchFileInput = document.createElement("input");
    switchFileInput.className = "form-check-input";
    switchFileInput.type = "checkbox";
    switchFileInput.id = "fileswitch1";
    switchFileInput.value = "link";
    if (isExternal === true) {
        switchFileInput.checked = false;
    } else {
        switchFileInput.checked = true;
    }
    switchFileUpperDiv.appendChild(switchFileInput);

    var switchFileLabel = document.createElement("label");
    switchFileLabel.className = "form-check-label";
    switchFileLabel.for = "fileswitch1";
    switchFileLabel.innerText = "Файл";
    switchFileUpperDiv.appendChild(switchFileLabel);

    switchLinkInput.onchange = function(){
        var blockId = switchLinkInput.parentNode.parentNode.parentNode.getAttribute("NBlockId");
        var newsId = switchLinkInput.parentNode.parentNode.parentNode.getAttribute("NNewsId");
        NewsLettersBlocks.forEach(function (block) {
            if (block.BlockNumber.toString() === blockId){
                block.NewsLetters.forEach(function (newsletter) {
                    if (newsletter.NewsNumber.toString() === newsId){
                        newsletter.ExternalLink = true;
                        newsletter.Image = "";
                        newsletter.ImageName = "";
                        if (switchFileInput.checked === true){
                            switchFileInput.checked = false;
                        }
                        RefreshNewsBlocks();
                    }
                });
            }
        });
    };
    switchFileInput.onchange = function(){
        var blockId = switchFileInput.parentNode.parentNode.parentNode.getAttribute("NBlockId");
        var newsId = switchFileInput.parentNode.parentNode.parentNode.getAttribute("NNewsId");
        NewsLettersBlocks.forEach(function (block) {
            if (block.BlockNumber.toString() === blockId){
                block.NewsLetters.forEach(function (newsletter) {
                    if (newsletter.NewsNumber.toString() === newsId){
                        newsletter.ExternalLink = false;
                        newsletter.Image = "";
                        newsletter.ImageName = "";
                        if (switchLinkInput.checked === true){
                            switchLinkInput.checked = false;
                        }
                        RefreshNewsBlocks();
                    }
                });
            }
        });
    };

    fileParagraph.appendChild(switchLinkUpperDiv);
    fileParagraph.appendChild(switchFileUpperDiv);
}

function AddFileFromLocalInput(newsletter) {
    newsletter.ExternalLink = false;
    var newsImage = document.createElement("input");
    newsImage.type = "file";
    newsImage.id = "newsImage" + newsletter.NewsNumber;
    newsImage.className = "custom-file-input";
    newsImage.required = true;
    newsImage.accept = "image/*";
    var newsImageLowerLabel = document.createElement("label");
    newsImageLowerLabel.className = "custom-file-label";
    newsImageLowerLabel.for = "newsImage" + newsletter.NewsNumber;
    if (newsletter.ImageName === "" || newsletter.ImageName == null){
        newsImageLowerLabel.innerHTML = "Выберите изображение";
    } else {
        newsImageLowerLabel.innerHTML = newsletter.ImageName;
    }
    return [newsImage, newsImageLowerLabel];
}



function AddFileExternalLink(block, newsletter) {
    newsletter.ExternalLink = true;
    var newsImage = document.createElement("input");
    newsImage.type = "text";
    newsImage.className = "form-control";
    if (newsletter.ImageLink == null || newsletter.ImageLink == undefined || newsletter.ImageLink == ""){
        newsImage.placeholder = "Ссылка...";
    } else {
        newsImage.value = newsletter.ImageLink;
    }
    newsImage.setAttribute("aria-describedby", "basic-addon2");

    newsImage.onchange = function (e) {
        var linkRequest = {Block: block.BlockNumber, NewsLetter: newsletter.NewsNumber, FileLink: e.target.value, FileData: ""};
        if (ws.readyState == 1){
            var message = {
                "messagetype": "ImageFromLinkRequest", "messagebody": JSON.stringify(linkRequest)
            };
            ws.send(JSON.stringify(message));
        }
    };

    return newsImage;
}

function ImageLinkResponseProc(imageResponse) {
    NewsLettersBlocks.forEach(function (block) {
        if (block.BlockNumber === imageResponse.Block){
            block.NewsLetters.forEach(function (newsletter) {
                if (newsletter.NewsNumber === imageResponse.NewsLetter){
                    newsletter.ExternalLink = true;
                    newsletter.ImageLink = imageResponse.FileLink;
                    newsletter.Image = imageResponse.FileData;
                    RefreshNewsBlocks();
                }
            });
        }
    });
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
    selectRcpGroups.multiple = true;
    selectRcpGroups.className = "form-control";
    selectRcpGroups.id = "recipient-select";

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
    selectRcpGroups.size = rowsCount;

    recepientHigherDiv.appendChild(recLabel);
    recepientHigherDiv.appendChild(selectRcpGroups);
    parentElem.appendChild(recepientHigherDiv)
}

function ShowDeliverySaveState(message) {
    var deliverStatePhar = document.getElementById("deliverstate");
    deliverStatePhar.innerText = message;
}

function SetTemplate(choise) {
    var parentElem = document.getElementById("dropdownMenuButton");
    parentElem.innerText = choise;
    selectedTemplateName = choise;
}

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
    var parentElem = document.getElementById("templates-change");
    while (parentElem.firstChild){
        parentElem.removeChild(parentElem.firstChild)
    }

    templatesFromServer.forEach(function (template, i) {
        var dropdownTemplate = document.createElement("a");
        dropdownTemplate.className = "dropdown-item";
        dropdownTemplate.innerText = template.TemplateName;
        dropdownTemplate.onclick = function(){
            SetTemplate(template.TemplateName);
        };
        parentElem.appendChild(dropdownTemplate);
    });
}

function ShowDeliver() {
    if (selectedTemplateName == null){
        alert('Необходимо выбрать темплейт.');
        return;
    }
    var recipientGroups = [];
    var selectGroups = document.getElementById("recipient-select");
    for (var i = 0, l = selectGroups.options.length, o; i < l; i++){
        o = selectGroups.options[i];
        if ( o.selected == true ){
            recipientGroups.push(o.value)
        }
    }
    if (document.getElementById("theme").value == null || recipientGroups.length == 0){
        alert("Заполните нужные для рассылки поля.");
        return;
    }

    if (document.getElementById("theme").value == "" || recipientGroups.length == 0){
        alert("Заполните нужные для рассылки поля.");
        return;
    }

    var MailNewsLetters = {NewsLettersBlocks: NewsLettersBlocks};

    MailNewsLetters.NewsLettersBlocks.forEach(function (block) {
        block.NewsLetters.forEach(function (newsletter){
            newsletter.NewsNumber = newsletter.NewsNumber.toString();
        });
        var blockNumStr = parseInt(block.BlockNumber, 10);
        block.BlockNumber = block.BlockNumber.toString();
    });



    var myObj = {
        "messagetype": "ShowDeliver", "messagebody": JSON.stringify({
            SenderMail: document.getElementById("sendermail").value,
            SenderName: document.getElementById("sender").value,
            MailTheme: document.getElementById("theme").value,
            Attachments: Attachments,
            MailNews: MailNewsLetters,
            RecipientGroups: recipientGroups,
            MailTemplateName: selectedTemplateName,
        })
    };

    DisableButtonByID("preview-button-elem");
    ShowDeliverySaveState("Генерируется страница предпросмотра...");
    if (ws.readyState == 1){
        ws.send(JSON.stringify(myObj));
    } else {
        ShowDeliverySaveState("Ошибка подключения по Websocket. Дождитесь переподключения и повторите действие.");
    }
}

function EnableButtonByID(id) {
    var button = document.getElementById(id);
    button.disabled = false;
}

function DisableButtonByID(id) {
    var button = document.getElementById(id);
    button.disabled = true;
}

function SaveDeliver() {
    if (selectedTemplateName == null){
        alert('Необходимо выбрать темплейт.');
        return;
    }
    var recipientGroups = [];
    var selectGroups = document.getElementById("recipient-select");
    for (var i = 0, l = selectGroups.options.length, o; i < l; i++){
        o = selectGroups.options[i];
        if ( o.selected == true ){
            recipientGroups.push(o.value)
        }
    }
    if (document.getElementById("theme").value == null || recipientGroups.length == 0){
        alert("Заполните нужные для рассылки поля.");
        return;
    }

    if (document.getElementById("theme").value === "" || recipientGroups.length == 0){
        alert("Заполните нужные для рассылки поля.");
        return;
    }

    var MailNewsLetters = {NewsLettersBlocks: NewsLettersBlocks};

    MailNewsLetters.NewsLettersBlocks.forEach(function (block) {
        block.NewsLetters.forEach(function (newsletter){
            newsletter.NewsNumber = newsletter.NewsNumber.toString();
        });
        var blockNumStr = parseInt(block.BlockNumber, 10);
        block.BlockNumber = block.BlockNumber.toString();
    });

    var myObj = {
        "messagetype": "DeliverSave", "messagebody": JSON.stringify({
            SenderMail: document.getElementById("sendermail").value,
            SenderName: document.getElementById("sender").value,
            MailTheme: document.getElementById("theme").value,
            Attachments: Attachments,
            MailNews: MailNewsLetters,
            RecipientGroups: recipientGroups,
            MailTemplateName: selectedTemplateName,
        })
    };
    DisableButtonByID("save-button-elem");
    ShowDeliverySaveState("Рассылка сохраняется...");
    if (ws.readyState == 1){
        ws.send(JSON.stringify(myObj));
    } else {
        ShowDeliverySaveState("Ошибка подключения по Websocket. Дождитесь переподключения и повторите действие.");
    }
}

function SaveDeliverAndSend() {
    if (selectedTemplateName == null){
        alert('Необходимо выбрать темплейт.');
        return;
    }
    var recipientGroups = [];
    var selectGroups = document.getElementById("recipient-select");
    for (var i = 0, l = selectGroups.options.length, o; i < l; i++){
        o = selectGroups.options[i];
        if ( o.selected == true ){
            recipientGroups.push(o.value)
        }
    }
    if (document.getElementById("theme").value == null || recipientGroups.length == 0){
        alert("Заполните нужные для рассылки поля.");
        return;
    }

    if (document.getElementById("theme").value === "" || recipientGroups.length == 0){
        alert("Заполните нужные для рассылки поля.");
        return;
    }

    var MailNewsLetters = {NewsLettersBlocks: NewsLettersBlocks};

    MailNewsLetters.NewsLettersBlocks.forEach(function (block) {
        block.NewsLetters.forEach(function (newsletter){
            newsletter.NewsNumber = newsletter.NewsNumber.toString();
        });
        var blockNumStr = parseInt(block.BlockNumber, 10);
        block.BlockNumber = block.BlockNumber.toString();
    });

    var myObj = {
        "messagetype": "DeliverSaveAndSend", "messagebody": JSON.stringify({
            SenderMail: document.getElementById("sendermail").value,
            SenderName: document.getElementById("sender").value,
            MailTheme: document.getElementById("theme").value,
            Attachments: Attachments,
            MailNews: MailNewsLetters,
            RecipientGroups: recipientGroups,
            MailTemplateName: selectedTemplateName,
        })
    };
    DisableButtonByID("send-button-elem");
    ShowDeliverySaveState("Подготовка к отправке рассылки...");
    if (ws.readyState == 1){
        ws.send(JSON.stringify(myObj));
    } else {
        ShowDeliverySaveState("Ошибка подключения по Websocket. Дождитесь переподключения и повторите действие.");
    }
}
