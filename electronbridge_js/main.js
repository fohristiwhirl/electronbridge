"use strict";

const alert = require("./alert");
const child_process = require("child_process");
const electron = require("electron");
const fs = require('fs');
const ipcMain = require("electron").ipcMain;
const readline = require("readline");
const windows = require("./windows");

const DEV_LOG_WINDOW_ID = -1;
const TARGET_APP = "app.exe";

electron.app.on("ready", () => {
	main();
});

function rebuild_menu(write_to_exe, registered_commands) {

	let template = [
		{
			label: "App",
			submenu: [
				{
					label: "About",
					click: () => alert("Electron Bridge: window manager for Golang via Electron"),
				},
				{
					type: "separator"
				},
				{
					role: "quit"
				},
			]
		},
		windows.make_submenu(),
		{
			label: "Dev",
			submenu: [
				{
					role: "toggledevtools"
				},
				{
					label: "Show Dev Log",
					click: () => windows.show(DEV_LOG_WINDOW_ID),
				},
				{
					type: "separator"
				},
				{
					label: "Backend Panic",
					click: () => {
						let output = {
							type: "panic",
							content: null,
						};
						write_to_exe(JSON.stringify(output));
					}
				},
				{
					label: "Backend Quit",
					click: () => {
						let output = {
							type: "quit",
							content: null,
						};
						write_to_exe(JSON.stringify(output));
					}
				},
			]
		},
	];

	if (registered_commands.length > 0) {

		template[0]["submenu"].push({
			type: "separator"
		});

		for (let n = 0; n < registered_commands.length; n++) {
			template[0]["submenu"].push({
				label: registered_commands[n],
				click: () => {
					let output = {
						type: "cmd",
						content: {cmd: registered_commands[n]},
					};
					write_to_exe(JSON.stringify(output));
				}
			});
		}
	}

	const menu = electron.Menu.buildFromTemplate(template);
	electron.Menu.setApplicationMenu(menu);
}

function main() {

	// Create our log window..................................................

	windows.new_window({
		uid: DEV_LOG_WINDOW_ID,
		page: "pages/log_simple.html",
		name: "Dev Log",
		width: 600,
		height: 400,
		starthidden: true,
		resizable: true,
	});

	function write_to_log(msg) {
		if (msg instanceof Error) {
			msg = msg.toString();
		}
		if (typeof(msg) === "object") {
			msg = JSON.stringify(msg, null, "  ");
		}
		msg = msg.toString();
		windows.update({
			uid: DEV_LOG_WINDOW_ID,
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

	let registered_commands = [];

	scanner.on("line", (line) => {
		let j = JSON.parse(line);

		// write_to_log(line)

		if (j.command === "new") {
			windows.new_window(j.content);
			rebuild_menu(write_to_exe, registered_commands);
		}

		if (j.command === "update") {
			windows.update(j.content);
		}

		if (j.command === "alert") {
			alert(j.content);
		}

		if (j.command === "allowquit") {
			windows.quit_now_possible();
		}

		if (j.command === "register") {
			registered_commands.push(j.content);
			rebuild_menu(write_to_exe, registered_commands);
		}
	});

	// Stderr messages from the compiled app...................................

	let stderr_scanner = readline.createInterface({
		input: exe.stderr,
		output: undefined,
		terminal: false
	});

	stderr_scanner.on("line", (line) => {
		write_to_log(line);
		windows.show(DEV_LOG_WINDOW_ID);
	});

	// Messages from the renderer..............................................

	ipcMain.on("keydown", (event, msg) => {

		let windobject = windows.get_windobject_from_event(event);

		if (windobject === undefined) {
			return;
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
			return;
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
			return;
		}

		let output = {
			type: "mouse",
			content: {
				down: true,
				uid: windobject.uid,
				x: msg.x,
				y: msg.y
			}
		};

		write_to_exe(JSON.stringify(output));
	});

	ipcMain.on("mouseup", (event, msg) => {

		let windobject = windows.get_windobject_from_event(event);

		if (windobject === undefined) {
			return;
		}

		let output = {
			type: "mouse",
			content: {
				down: false,
				uid: windobject.uid,
				x: msg.x,
				y: msg.y
			}
		};

		write_to_exe(JSON.stringify(output));
	});

	ipcMain.on("mouseover", (event, msg) => {

		let windobject = windows.get_windobject_from_event(event);

		if (windobject === undefined) {
			return;
		}

		let output = {
			type: "mouseover",
			content: {
				uid: windobject.uid,
				x: msg.x,
				y: msg.y
			}
		};

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

	ipcMain.on("log", (event, opts) => {
		let windobject = windows.get_windobject_from_event(event);
		write_to_log("Log from window '" + windobject.config.name + "': " + opts.msg);
	});
}
