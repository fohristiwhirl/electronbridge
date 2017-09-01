"use strict";

// The animators here use absolute "real world" grid coordinates
// as their internal x and y coordinates. We can convert these to
// screen coordinates based on renderer.camerax and .cameray.
// We also have to take into account the left and top borders.

const canvas = document.getElementById("canvas");
const virtue = canvas.getContext("2d");

const in_canvas = (x, y) => {
	if (x < 0 || x >= canvas.width || y < 0 || y >= canvas.height) {
		return false;
	}
	return true;
};

const int_rand_range = (min, max) => {
	min = Math.ceil(min);
	max = Math.floor(max);
	return Math.floor(Math.random() * (max - min)) + min;
};

const NULL_ANIMATOR = {		// This is sort of a reference object. Every animator must have these 2 things.
	finished: true,
	step: () => null,
};

exports.make_shot = (opts, renderer) => {

	if (opts.x1 === opts.x2 && opts.y1 === opts.y2) {
		return NULL_ANIMATOR;
	}

	let frame = 0;

	if (opts.duration <= 0) {
		opts.duration = 1;
	}

	let x = opts.x1;
	let y = opts.y1;

	let colour = `rgb(${opts.r}, ${opts.g}, ${opts.b})`;

	let frame_dx = (opts.x2 - opts.x1) / opts.duration;
	let frame_dy = (opts.y2 - opts.y1) / opts.duration;

	let that = Object.create(null);
	that.finished = false;
	that.step = () => {

		frame++;

		if (frame > opts.duration) {
			that.finished = true;
			return;
		}

		let next_x = x + frame_dx;
		let next_y = y + frame_dy;

		let [x1p, y1p] = renderer.animation_pixel_xy_from_world_xy(x, y);
		let [x2p, y2p] = renderer.animation_pixel_xy_from_world_xy(next_x, next_y);

		if (in_canvas(x1p, y1p) && in_canvas(x2p, y2p)) {
			virtue.strokeStyle = colour;
			virtue.beginPath();
			virtue.moveTo(x1p, y1p);
			virtue.lineTo(x2p, y2p);
			virtue.stroke();
		} else {
			that.finished = true;
			return;
		}

		x = next_x;
		y = next_y;
	};

	return that;
};

exports.make_flash = (opts, renderer) => {

	let frame = 0;

	let r = Math.floor(opts.r);
	let g = Math.floor(opts.g);
	let b = Math.floor(opts.b);

	let that = Object.create(null);
	that.finished = false;
	that.step = () => {

		frame++;

		if (frame > opts.duration) {
			that.finished = true;
			return;
		}

		let [i, j] = renderer.animation_pixel_xy_from_world_xy(opts.x, opts.y);

		if (in_canvas(i, j)) {
			let a = ((opts.duration - frame) / opts.duration) * opts.opacity;
			virtue.fillStyle = `rgba(${r}, ${g}, ${b}, ${a})`;
			virtue.fillRect(i - renderer.true_boxwidth / 2, j - renderer.true_boxheight / 2, renderer.true_boxwidth, renderer.true_boxheight);
		} else {
			that.finished = true;
			return;
		}
	};

	return that;
};

exports.make_explosion = (opts, renderer) => {

	let frame = 0;
	let cells = [];

	for (let i = -opts.radius; i <= opts.radius; i++) {

		for (let j = -opts.radius; j <= opts.radius; j++) {

			if (Math.sqrt(i * i + j * j) <= opts.radius + 0.25) {

				let sub_opts = {
					r: int_rand_range(192, 256),
					g: int_rand_range(64, 128),
					b: int_rand_range(0, 64),
					x: opts.x + i,
					y: opts.y + j,
					duration: opts.duration,
					opacity: 1.0,
				};

				cells.push(exports.make_flash(sub_opts, renderer));
			}
		}
	}

	let that = Object.create(null);
	that.finished = false;
	that.step = () => {

		frame++;

		if (frame > opts.duration) {
			that.finished = true;
			return;
		}

		for (let n = 0; n < cells.length; n++) {
			cells[n].step();
		}
	};

	return that;
};

exports.make_cascade = (opts, renderer) => {

	let frame = 0;
	let cells = [];

	let linelength = opts.points.length;
	let lastindex = -1;

	let that = Object.create(null);
	that.finished = false;
	that.step = () => {

		frame++;

		let i = Math.floor(linelength * (frame / opts.duration));
		if (i > lastindex && i < linelength) {

			lastindex = i;

			let sub_opts = {
				r: opts.r,
				g: opts.g,
				b: opts.b,
				x: opts.points[i].x,
				y: opts.points[i].y,
				duration: opts.duration,
				opacity: opts.opacity,
			};

			cells.push(exports.make_flash(sub_opts, renderer));
		}

		for (let n = 0; n < cells.length; n++) {
			cells[n].step();
		}

		if (cells.length > 0) {

			that.finished = true;

			for (let n = 0; n < cells.length; n++) {
				if (cells[n].finished === false) {
					that.finished = false;
				}
			}
		}
	};

	return that;
};
