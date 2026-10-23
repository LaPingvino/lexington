// Lexington's page: the WebAssembly (lexington.wasm) does the work.
(async () => {
	const $ = (id) => document.getElementById(id);
	const status = (text) => { $("status").textContent = text; };
	const go = new Go();
	try {
		const wasm = await WebAssembly.instantiateStreaming(fetch("lexington.wasm"), go.importObject);
		go.run(wasm.instance);
	} catch (e) {
		status("Lexington could not start: " + e);
		return;
	}
	for (const f of JSON.parse(lexington.formats())) {
		$("to").add(new Option(f.Name, f.Key));
	}
	const presets = JSON.parse(lexington.presets());
	for (const p of presets) {
		$("elements").add(new Option(p.Name, p.Key));
	}
	const about = () => {
		const p = presets.find((p) => p.Key === $("elements").value);
		$("elements-about").textContent = p ? p.Description : "";
	};
	$("elements").addEventListener("change", about);
	about();
	$("convert").textContent = "Convert";
	$("convert").disabled = false;

	$("form").addEventListener("submit", async (e) => {
		e.preventDefault();
		const file = $("file").files[0];
		if (!file) return;
		status("Converting " + file.name + "…");
		await new Promise((r) => setTimeout(r)); // show it before the work
		const data = new Uint8Array(await file.arrayBuffer());
		const r = lexington.convert(file.name, data, $("to").value, $("elements").value, $("page").value);
		if (r.error) {
			status("Could not convert " + file.name + ": " + r.error);
			return;
		}
		const ext = JSON.parse(lexington.formats()).find((f) => f.Key === $("to").value).Ext;
		const name = file.name.replace(/\.[^.]*$/, "") + "." + ext;
		const a = document.createElement("a");
		a.href = URL.createObjectURL(new Blob([r.data]));
		a.download = name;
		a.click();
		URL.revokeObjectURL(a.href);
		status("Converted: " + name);
	});
})();
