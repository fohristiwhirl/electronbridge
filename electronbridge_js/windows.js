"use strict";

const alert = require("./alert");
const assert = require("assert");
const electron = require("electron");
const fs = require("fs");
const url = require("url");

// The windobject is our fundamental object, containing fields:
//			{uid, win, config, ready, queue}

let windobjects = Object.create(null);		// dict: uid --> windobject

let quit_possible = false;					// call quit_now_possible() to set this true and allow the module to quit the app

exports.get_windobject_from_event = (event) => {
	for (let uid in windobjects) {
		let windobject = windobjects[uid];
		if (windobject.win.webContents === event.sender) {
			return windobject;
		}
	}
	return undefined;
};

exports.resize = (windobject, opts) => {
	if (windobject) {
		windobject.win.setContentSize(Math.floor(opts.xpixels), Math.floor(opts.ypixels));
	}
};

exports.new_window = (config) => {

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
		width: Math.floor(win_pixel_width),
		height: Math.floor(win_pixel_height),
		backgroundColor: "#000000",
		useContentSize: true,
		resizable: config.resizable,
		webPreferences: {
			nodeIntegration: true,
			webSecurity: false
		}
	});

	let f = url.format({
		protocol: "file:",
		pathname: config.page,
		slashes: true
	})

	console.log(f);

	win.loadURL(f);

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
};

exports.relay = (channel, content) => {
	let windobject = windobjects[content.uid];
	send_or_queue(windobject, channel, content);
};

exports.handle_ready = (windobject, opts) => {

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
};

exports.hide = (uid) => {
	let windobject = windobjects[uid];
	if (windobject === undefined) {
		return;
	}
	try {
		windobject.win.hide();
	} catch (e) {
		// Can fail at end of app life when the window has been destroyed.
	}
};

exports.show = (uid) => {
	let windobject = windobjects[uid];
	if (windobject === undefined) {
		return;
	}
	try {
		windobject.win.show();
	} catch (e) {
		// Can fail at end of app life when the window has been destroyed.
	}
};

exports.screenshot = (uid) => {
	let windobject = windobjects[uid];
	if (windobject === undefined) {
		return;
	}

	let c = windobject.win.webContents;
	c.capturePage((image) => {
		if (fs.existsSync("screenshots") == false) {
    		fs.mkdirSync("screenshots");
		}
		let buffer = image.toPNG();
		fs.writeFile(`screenshots/screenshot_${Date.now()}.png`, buffer, () => null);
	});
};

exports.quit_now_possible = () => {
	quit_possible = true;
};

exports.make_window_menu_items = () => {

	let items = [];
	let uids = positive_uids();

	for (let uid of uids) {

		let win_name = windobjects[uid].config.name;

		items.push({
			label: win_name,
			click: () => exports.show(uid),
		});
	}

	return items;
};

exports.make_screenshot_menu_items = () => {

	let items = [];
	let uids = positive_uids();

	for (let uid of uids) {

		let win_name = windobjects[uid].config.name;

		items.push({
			label: "Screenshot: " + win_name,
			click: () => exports.screenshot(uid),
		});
	}

	return items;
};

// --------------------------------------------------------------------------

function send_or_queue(windobject, channel, msg) {
	if (windobject === undefined) {
		return;
	}
	/*
	if (windobject.ready !== true) {
		windobject.queue.push(() => windobject.send(channel, msg));
		console.log("queued")
		return;
	}
	*/
	try {
		windobject.send(channel, msg);
	} catch (e) {
		// Can fail at end of app life when the window has been destroyed.
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

	electron.app.exit();						// Why doesn't quit work?
}

function all_uids(positive_only) {

	let all_uids = [];

	for (let uid in windobjects) {
		if (!positive_only || uid > 0) {
			all_uids.push(parseInt(uid));		// Stupid JS will have converted the uid key to a string.
		}
	}

	all_uids.sort((a, b) => {return a - b;});

	return all_uids;
}

function positive_uids() {
	return all_uids(true);
}
