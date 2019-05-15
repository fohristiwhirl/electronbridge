"use strict";

const alert = require("./alert");
const child_process = require("child_process");
const electron = require("electron");
const fs = require('fs');
const ipcMain = require("electron").ipcMain;
const readline = require("readline");
const windows = require("./windows");

const DEV_LOG_WINDOW_ID = -1;
const TARGET_APP = "./app";

let about_message = `Electron Bridge: window manager for Golang via Electron\n` +
					`--\n` +
					`Electron ${process.versions.electron}\n` +
					`Node ${process.versions.node}\n` +
					`V8 ${process.versions.v8}`

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
					click: () => alert(about_message),
				},
				{
					type: "separator",
				},
				{
					role: "quit",
				},
			]
		},
		{
			label: "Windows",
			submenu: windows.make_window_menu_items(),
		},
		{
			label: "Dev",
			submenu: [
				{
					role: "toggledevtools",
				},
				{
					label: "Show Dev Log",
					click: () => windows.show(DEV_LOG_WINDOW_ID),
				},
				{
					type: "separator",
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
				{
					type: "separator",
				},
			]
		},
	];

	let screenshot_items = windows.make_screenshot_menu_items();

	for (let n = 0; n < screenshot_items.length; n++) {
		let item = screenshot_items[n];
		// let a = n + 1;
		// item.accelerator = a.toString();
		template[2]["submenu"].push(item);
	}

	if (registered_commands.length > 0) {
		template[0]["submenu"].push({type: "separator"});
	}

	for (let n = 0; n < registered_commands.length; n++) {
		template[0]["submenu"].push(registered_commands[n]);
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
		width: 800,
		height: 600,
		starthidden: true,
		resizable: true,
	});

	let have_warned_socket = false;

	function write_to_log(sender, msg) {
		if (msg instanceof Error) {
			msg = msg.toString();
		}
		if (typeof(msg) === "object") {
			msg = JSON.stringify(msg, null, "  ");
		}
		if (typeof(msg) === "undefined") {
			msg = "undefined";
		}
		msg = msg.toString();

		if (msg.indexOf("Error: This socket has been ended by the other party") !== -1) {
			if (have_warned_socket) {
				return;
			}
			have_warned_socket = true;
		}

		windows.relay("update", {
			uid: DEV_LOG_WINDOW_ID,
			msg: sender + ":  " + msg + "\n",
		});
	}

	// Communications with the compiled app...................................

	let exe = child_process.spawn(TARGET_APP);

	write_to_log("main.js", "Connected to " + TARGET_APP);

	function write_to_exe(msg) {
		try {
			exe.stdin.write(msg + "\n");
		} catch (e) {
			write_to_log("main.js", e);
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

		if (j.command === "new") {
			windows.new_window(j.content);
		}

		if (j.command === "update" || j.command === "effect") {
			windows.relay(j.command, j.content);
		}

		if (j.command === "alert") {
			alert(j.content);
		}

		if (j.command === "allowquit") {
			windows.quit_now_possible();
		}

		if (j.command === "register") {

			let item = {
				label: j.content.label,
				accelerator: j.content.accelerator === "" ? undefined : j.content.accelerator,
				click: () => {
					let output = {
						type: "cmd",
						content: {cmd: j.content.label},
					};
					write_to_exe(JSON.stringify(output));
				}
			};

			registered_commands.push(item);
		}

		if (j.command === "separator") {
			registered_commands.push({type: "separator"});
		}

		if (j.command === "buildmenu") {
			rebuild_menu(write_to_exe, registered_commands);
		}

		if (j.command === "about") {
			about_message = `${j.content}\n` +
							`--\n` +
							`Electron ${process.versions.electron}\n` +
							`Node ${process.versions.node}\n` +
							`V8 ${process.versions.v8}`;
		}

		if (j.command === "front") {
			windows.show(j.content);
		}

		if (j.command === "silentlog") {
			write_to_log(TARGET_APP, j.content);
		}
	});

	// Stderr messages from the compiled app...................................

	let stderr_scanner = readline.createInterface({
		input: exe.stderr,
		output: undefined,
		terminal: false
	});

	stderr_scanner.on("line", (line) => {
		write_to_log(TARGET_APP, line);
		windows.show(DEV_LOG_WINDOW_ID);
	});

	// Messages from the renderer..............................................

	ipcMain.on("ack", (event, msg) => {
		let output = {
			type: "ack",
			content: {
				AckMessage: msg
			}
		};
		write_to_exe(JSON.stringify(output));
	});

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
				y: msg.y,
				button: msg.button
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
				y: msg.y,
				button: msg.button
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
		write_to_log(windobject.config.name, opts.msg);
	});

	ipcMain.on("screenshot", (event, opts) => {
		let windobject = windows.get_windobject_from_event(event);
		windows.screenshot(windobject.uid);
	});

	ipcMain.on("error", (event, opts) => {
		let windobject = windows.get_windobject_from_event(event);
		write_to_log(windobject.config.name + " (ERROR)", opts.msg);
		windows.show(DEV_LOG_WINDOW_ID);
	});
}
