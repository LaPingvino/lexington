// The Formats page: web/formats.json, which cli/matrix_test.go makes, as
// a table.
(async () => {
	const $ = (id) => document.getElementById(id);
	const marks = { survives: "✓", degrades: "~", lost: "✗" };
	const words = { survives: "survives", degrades: "degrades", lost: "is lost" };
	// a note's `code` as code, the rest as text
	const note = (el, text) => {
		text.split("`").forEach((part, i) => {
			if (i % 2) {
				const c = document.createElement("code");
				c.textContent = part;
				el.append(c);
			} else {
				el.append(part);
			}
		});
	};
	let m;
	try {
		const res = await fetch("formats.json");
		if (!res.ok) throw new Error(res.status + " " + res.statusText);
		m = await res.json();
	} catch (e) {
		$("matrix").querySelector("caption").textContent = "The table could not be loaded: " + e.message;
		return;
	}
	const table = $("matrix");
	table.replaceChildren();
	const head = table.createTHead().insertRow();
	for (const h of ["Format", "Read", "Write", ...m.features.map((f) => f.name)]) {
		const th = document.createElement("th");
		th.scope = "col";
		th.textContent = h;
		head.append(th);
	}
	const body = table.createTBody();
	const notes = [];
	const only = [];
	for (const f of m.formats) {
		if (!f.features) {
			only.push(f.needs ? f.name + " (" + f.needs + ")" : f.name);
			continue;
		}
		const row = body.insertRow();
		const th = document.createElement("th");
		th.scope = "row";
		th.textContent = f.name;
		row.append(th);
		for (const yes of [f.read, f.write]) row.insertCell().textContent = yes ? "✓" : "✗";
		for (const ft of m.features) {
			const cell = row.insertCell();
			const c = f.features[ft.key];
			if (!c) continue;
			cell.className = c.status;
			cell.textContent = marks[c.status];
			cell.title = ft.name + " " + words[c.status] + (c.note ? ": " + c.note.replaceAll("`", "") : "");
			if (c.note) {
				let i = notes.indexOf(c.note);
				if (i < 0) i = notes.push(c.note) - 1;
				const sup = document.createElement("sup");
				sup.textContent = i + 1;
				cell.append(sup);
			}
		}
	}
	for (const n of notes) note($("notes").appendChild(document.createElement("li")), n);
	$("only").textContent = only.join(", ") + ".";
})();
