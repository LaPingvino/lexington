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
	// the settings (lexington.toml), kept in this browser
	const stored = () => { try { return localStorage.getItem("lexington.toml") || ""; } catch { return ""; } };
	let elements = [];
	const fill = (select, items, keepFirst) => {
		const was = select.value;
		while (select.options.length > keepFirst) select.remove(keepFirst);
		for (const c of items) select.add(new Option(c.Name, c.Key));
		if ([...select.options].some((o) => o.value === was)) select.value = was;
	};
	const loadChoices = () => {
		const c = JSON.parse(lexington.choices(stored()));
		elements = c.Elements || [];
		fill($("elements"), elements, 0);
		fill($("scenein"), c.Scenes || [], 1);
		fill($("sceneout"), c.Scenes || [], 1);
		about();
	};
	const about = () => {
		const p = elements.find((p) => p.Key === $("elements").value);
		$("elements-about").textContent = p ? p.Description : "";
	};
	$("settings").value = stored() || lexington.defaultSettings();
	$("save-settings").addEventListener("click", () => {
		try { localStorage.setItem("lexington.toml", $("settings").value); } catch {}
		loadChoices();
		status("Settings saved in this browser.");
	});
	$("reset-settings").addEventListener("click", () => {
		try { localStorage.removeItem("lexington.toml"); } catch {}
		$("settings").value = lexington.defaultSettings();
		loadChoices();
		status("Back to Lexington's default settings.");
	});
	$("elements").addEventListener("change", about);
	loadChoices();
	$("convert").textContent = "Convert";
	$("convert").disabled = false;

	$("form").addEventListener("submit", async (e) => {
		e.preventDefault();
		const file = $("file").files[0];
		if (!file) return;
		status("Converting " + file.name + "…");
		await new Promise((r) => setTimeout(r)); // show it before the work
		const data = new Uint8Array(await file.arrayBuffer());
		const r = lexington.convert(file.name, data, $("to").value, $("elements").value, $("page").value,
			$("scenein").value, $("sceneout").value, stored());
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
