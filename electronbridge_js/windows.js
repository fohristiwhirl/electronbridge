"use strict";

const alert = require("./alert");
const assert = require("assert");
const electron = require("electron");
const url = require("url");

// The windobject is our fundamental object, containing fields:
//			{uid, win, config, ready, queue}

let windobjects = Object.create(null);		// dict: uid --> windobject

let quit_possible = false;					// call quit_now_possible() to set this true and allow the module to quit the app

function get_windobject_from_event(event) {
	for (let uid in windobjects) {
		let val = windobjects[uid];
		if (val.win.webContents === event.sender) {
			return val;
		}
	}
	return undefined;
}

function resize(windobject, opts) {
	if (windobject) {
		windobject.win.setContentSize(opts.xpixels, opts.ypixels);
	}
}

function new_window(config) {

	assert(config.uid !== undefined);
	assert(windobjects[config.uid] === undefined);

	let win_pixel_width = config.width;
	let win_pixel_height = config.height;

	// The config may or may not specify width and height in terms of a grid of boxes, with each box taking up a certain size...

	if (config.boxwidth !== undefined && config.boxheight !== undefined) {
		win_pixel_width *= config.boxwidth;
		win_pixel_height *= config.boxheight;
	}

	let win = new electron.BrowserWindow({
		show: false,
		title: config.name,
		width: win_pixel_width,
		height: win_pixel_height,
		backgroundColor: "#000000",
		useContentSize: true,
		resizable: config.resizable
	});

	win.loadURL(url.format({
		protocol: "file:",
		pathname: config.page,
		slashes: true
	}));

	if (config.nomenu === true) {
		win.setMenu(null);
	}

	if (config.starthidden !== true) {
		win.show();
	}

	win.on("close", (evt) => {
		evt.preventDefault();
		win.hide();
		quit_if_all_windows_are_hidden();
	});

	win.on("hide", () => {
		quit_if_all_windows_are_hidden();
	});

	windobjects[config.uid] = {
		uid: config.uid,
		win: win,
		config: config,
		ready: false,
		queue: [],

		send: (channel, msg) => {
			win.webContents.send(channel, msg);
		}
	};
}

function update(content) {
	let windobject = windobjects[content.uid];
	send_or_queue(windobject, "update", content);
}

function send_or_queue(windobject, channel, msg) {
	if (windobject === undefined) {
		return;
	}
	if (windobject.ready !== true) {
		windobject.queue.push(() => windobject.send(channel, msg));
		return;
	}
	try {
		windobject.send(channel, msg);
	} catch (e) {
		// Can fail at end of app life when the window has been destroyed.
	}
}

function handle_ready(windobject, opts) {

	if (windobject === undefined) {
		return;
	}

	windobject.ready = true;

	let config = windobject.config;
	windobject.send("init", config);

	// Now carry out whatever actions were delayed because the window wasn't ready...

	for (let n = 0; n < windobject.queue.length; n++) {
		windobject.queue[n]();
	}

	windobject.queue = [];
}

function hide(uid) {
	let windobject = windobjects[uid];
	if (windobject === undefined) {
		return;
	}
	try {
		windobject.win.hide();
	} catch (e) {
		// Can fail at end of app life when the window has been destroyed.
	}
}

function show(uid) {
	let windobject = windobjects[uid];
	if (windobject === undefined) {
		return;
	}
	try {
		windobject.win.show();
	} catch (e) {
		// Can fail at end of app life when the window has been destroyed.
	}
}

function show_all_except(uid_array) {

	let keys = Object.keys(windobjects);
	let exceptions = Object.create(null);

	for (let n = 0; n < uid_array.length; n++) {
		exceptions[uid_array[n]] = true;
	}

	for (let n = 0; n < keys.length; n++) {
		let key = keys[n];
		let windobject = windobjects[key];

		if (exceptions[windobject.uid] !== true) {
			windobject.win.show();
		}
	}
}

function quit_if_all_windows_are_hidden() {

	if (!quit_possible) {
		return;
	}

	let keys = Object.keys(windobjects);

	for (let n = 0; n < keys.length; n++) {

		let key = keys[n];

		let windobject = windobjects[key];

		try {
			if (windobject.win.isVisible()) {
				return;
			}
		} catch (e) {
			// Can fail at end of app life when the window has been destroyed.
		}
	}

	electron.app.exit();		// Why doesn't quit work?
}

function quit_now_possible() {
	quit_possible = true;
}

function make_submenu() {

	let ret = {
		label: "Windows",
		submenu: [],
	};

	let keys = Object.keys(windobjects);

	for (let n = 0; n < keys.length; n++) {

		let key = keys[n];

		if (key < 0) {
			continue;
		}

		let win_name = windobjects[key].config.name;

		ret.submenu.push({
			label: win_name,
			click: () => show(key)
		});
	}

	return ret;
}

exports.get_windobject_from_event = get_windobject_from_event;
exports.resize = resize;
exports.new_window = new_window;
exports.update = update;
exports.handle_ready = handle_ready;
exports.hide = hide;
exports.show = show;
exports.show_all_except = show_all_except;
exports.quit_now_possible = quit_now_possible;
exports.make_submenu = make_submenu;
