"use strict";

const alert = require("./alert");
const child_process = require("child_process");
const electron = require("electron");
const fs = require('fs');
const ipcMain = require("electron").ipcMain;
const readline = require("readline");
const windows = require("./windows");

const STDERR_LOG_WINDOW_ID = -1
const TARGET_APP = "app.exe"


electron.app.on("ready", () => {
	main();
});


function main() {

	// Menu...................................................................

	const template = [
		{
			label: "Game",
			submenu: [
				{
					role: "quit"
				},
				{
					type: "separator"
				},
				{
					label: "Show All App Windows",
					click: () => windows.show_all_except([STDERR_LOG_WINDOW_ID])
				},
			]
		},
		{
			label: "Dev",
			submenu: [
				{
					role: "toggledevtools"
				},
				{
					label: "Show Client Log",
					click: () => windows.show(STDERR_LOG_WINDOW_ID),
				},
				{
					type: "separator"
				},
				{
					label: "Panic",
					click: () => {
						let output = {
							type: "panic",
							content: null,
						}
						write_to_exe(JSON.stringify(output));
					}
				},
			]
		},
	];

	const menu = electron.Menu.buildFromTemplate(template);
	electron.Menu.setApplicationMenu(menu);

	// Create our log window..................................................

	windows.new_window({
		uid: STDERR_LOG_WINDOW_ID,
		page: "pages/log_simple.html",
		name: "Client Log",
		width: 600,
		height: 400,
		resizable: true,
	});

	windows.hide(STDERR_LOG_WINDOW_ID);

	function write_to_log(msg) {
		if (msg instanceof Error) {
			msg = msg.toString();
		}
		if (typeof(msg) === "object") {
			msg = JSON.stringify(msg, null, "  ");
		}
		msg = msg.toString();
		windows.update({
			uid: STDERR_LOG_WINDOW_ID,
			msg: msg + "\n",
		});
	}

	// Communications with the compiled app...................................

	let exe = child_process.spawn(TARGET_APP);

	write_to_log("Connected to " + TARGET_APP);

	function write_to_exe(msg) {
		try {
			exe.stdin.write(msg + "\n");
		} catch (e) {
			write_to_log(e);
		}
	}

	let scanner = readline.createInterface({
		input: exe.stdout,
		output: undefined,
		terminal: false
	});

	scanner.on("line", (line) => {
		let j = JSON.parse(line);

		// write_to_log(line)

		if (j.command === "new") {
			windows.new_window(j.content);
			windows.quit_now_possible();			// Tell windows module that quitting the app is allowed (i.e. if all windows get closed).
		}

		if (j.command === "update") {
			windows.update(j.content);
		}

		if (j.command === "alert") {
			alert(j.content);
		}
	});

	// Stderr messages from the compiled app...................................

	let stderr_scanner = readline.createInterface({
		input: exe.stderr,
		output: undefined,
		terminal: false
	});

	stderr_scanner.on("line", (line) => {
		write_to_log(line)
	});

	// Messages from the renderer..............................................

	ipcMain.on("keydown", (event, msg) => {

		let windobject = windows.get_windobject_from_event(event);

		if (windobject === undefined) {
			return
		}

		let output = {
			type: "key",
			content: {
				down: true,
				uid: windobject.uid,
				key: msg.key
			}
		};

		write_to_exe(JSON.stringify(output));
	});

	ipcMain.on("keyup", (event, msg) => {

		let windobject = windows.get_windobject_from_event(event);

		if (windobject === undefined) {
			return
		}

		let output = {
			type: "key",
			content: {
				down: false,
				uid: windobject.uid,
				key: msg.key
			}
		};

		write_to_exe(JSON.stringify(output));
	});

	ipcMain.on("mousedown", (event, msg) => {

		let windobject = windows.get_windobject_from_event(event);

		if (windobject === undefined) {
			return
		}

		let output = {
			type: "mouse",
			content: {
				down: true,
				uid: windobject.uid,
				x: msg.x,
				y: msg.y
			}
		}

		write_to_exe(JSON.stringify(output));
	});

	ipcMain.on("mouseup", (event, msg) => {

		let windobject = windows.get_windobject_from_event(event);

		if (windobject === undefined) {
			return
		}

		let output = {
			type: "mouse",
			content: {
				down: false,
				uid: windobject.uid,
				x: msg.x,
				y: msg.y
			}
		}

		write_to_exe(JSON.stringify(output));
	});

	ipcMain.on("request_resize", (event, opts) => {
		let windobject = windows.get_windobject_from_event(event);
		windows.resize(windobject, opts);
	});

	ipcMain.on("ready", (event, opts) => {
		let windobject = windows.get_windobject_from_event(event);
		windows.handle_ready(windobject, opts);
	});

	ipcMain.on("effect_done", (event, opts) => {
		let windobject = windows.get_windobject_from_event(event);
		let output = {
			type: "effect_done",
			content: {
				uid: windobject.uid,
				effectid: opts.effectid,
			}
		}
		write_to_exe(JSON.stringify(output));
	});
}
