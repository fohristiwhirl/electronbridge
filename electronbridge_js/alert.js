"use strict";

const electron = require("electron");

function object_to_string(o) {
    if (typeof(o) === "undefined") {
        return "undefined";
    }
    let msg = JSON.stringify(o);
    return msg;
}

function alert_main(msg) {
    electron.dialog.showMessageBox({
        message: msg.toString(),
        title: "Alert",
        buttons: ["OK"]
    }, () => {});               // Providing a callback makes the window not block the process
}

function alert_renderer(msg) {
    electron.remote.dialog.showMessageBox({
        message: msg.toString(),
        title: "Alert",
        buttons: ["OK"]
    }, () => {});
}

module.exports = (msg) => {
    if (typeof(msg) !== "string") {
        msg = object_to_string(msg);
    }
    msg = msg.trim();
    if (process.type === "renderer") {
        alert_renderer(msg);
    } else {
        alert_main(msg);
    }
}
