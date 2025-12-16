function showMessage(text, isError = false) {
    const el = document.getElementById("message");
    if (!el) return;

    if (!text) {
        el.classList.add("d-none");
        el.textContent = "";
        el.classList.remove("alert-danger", "alert-success");
        return;
    }

    el.textContent = text;
    el.classList.remove("d-none", "alert-danger", "alert-success");
    el.classList.add(isError ? "alert-danger" : "alert-success");
}

/** Состояние страницы */
const importState = {
    licensesForImport: [] // { key }
};

// ---------- утилиты чтения файлов ----------

function parseCSV(text) {
    const rows = [];
    const lines = text.split(/\r?\n/).filter((line) => line.trim() !== "");
    for (const line of lines) {
        const parts = line.includes(";") ? line.split(";") : line.split(",");
        rows.push(parts.map((x) => x.trim()));
    }
    return rows;
}

/** Берём "первый непустой столбец" каждой строки и возвращаем массив строк. */
function extractFirstColumnValues(rows) {
    const result = [];
    for (const row of rows) {
        if (!row || !row.length) continue;
        let value = "";
        for (let i = 0; i < row.length; i++) {
            const cell = String(row[i] ?? "").trim();
            if (cell !== "") {
                value = cell;
                break;
            }
        }
        if (value) result.push(value);
    }
    return result;
}

async function readFileAsRows(file) {
    const ext = file.name.split(".").pop().toLowerCase();

    if (ext === "csv" || ext === "txt") {
        const text = await file.text();
        return parseCSV(text);
    } else {
        const data = await file.arrayBuffer();
        const workbook = XLSX.read(data, { type: "array" });
        const sheetName = workbook.SheetNames[0];
        const sheet = workbook.Sheets[sheetName];
        return XLSX.utils.sheet_to_json(sheet, { header: 1, raw: false });
    }
}

// ---------- ключи ----------

function renderLicensesTable() {
    const table = document.getElementById("licenses-table");
    if (!table) return;

    table.innerHTML = "";

    const thead = document.createElement("thead");
    const trHead = document.createElement("tr");
    ["Ключ", "Действия"].forEach((t) => {
        const th = document.createElement("th");
        th.textContent = t;
        trHead.appendChild(th);
    });
    thead.appendChild(trHead);
    table.appendChild(thead);

    const tbody = document.createElement("tbody");

    importState.licensesForImport.forEach((l, index) => {
        const tr = document.createElement("tr");

        const tdKey = document.createElement("td");
        const inpKey = document.createElement("input");
        inpKey.type = "text";
        inpKey.className = "form-control form-control-sm";
        inpKey.value = l.key || "";
        inpKey.placeholder = "Лицензионный ключ";
        inpKey.addEventListener("input", () => {
            importState.licensesForImport[index].key = inpKey.value;
        });
        tdKey.appendChild(inpKey);
        tr.appendChild(tdKey);

        const tdAct = document.createElement("td");
        const btnDel = document.createElement("button");
        btnDel.textContent = "Удалить";
        btnDel.className = "btn btn-sm btn-outline-danger";
        btnDel.addEventListener("click", () => {
            importState.licensesForImport.splice(index, 1);
            renderLicensesTable();
        });
        tdAct.appendChild(btnDel);
        tr.appendChild(tdAct);

        tbody.appendChild(tr);
    });

    table.appendChild(tbody);
}

function addLicenseRow() {
    importState.licensesForImport.push({ key: "" });
    renderLicensesTable();
}

function clearLicenses() {
    importState.licensesForImport = [];
    renderLicensesTable();
}

async function loadLicensesFromFile(file) {
    try {
        const rows = await readFileAsRows(file);
        const values = extractFirstColumnValues(rows);
        let addedCount = 0;

        values.forEach((v) => {
            if (!v) return;
            const exists = importState.licensesForImport.some(
                (l) => (l.key || "").trim().toLowerCase() === v.toLowerCase()
            );
            if (!exists) {
                importState.licensesForImport.push({ key: v });
                addedCount++;
            }
        });

        renderLicensesTable();
        showMessage(`Из файла добавлено ключей: ${addedCount}`, false);
    } catch (e) {
        console.error(e);
        showMessage("Ошибка чтения файла ключей: " + e.message, true);
    }
}

// ---------- сохранение в бэкенд ----------

async function saveToServer() {
    importState.licensesForImport = importState.licensesForImport.filter(
        (l) => (l.key || "").trim() !== ""
    );

    if (!importState.licensesForImport.length) {
        showMessage("Нет данных для сохранения.", true);
        return;
    }

    showMessage("Сохраняем данные…", false);

    try {
        const licensesPayload = importState.licensesForImport.map((l) => ({
            key: l.key,
            comment: "",
            pc: ""
        }));
        const res = await fetch("/api/licenses/import", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ licenses: licensesPayload })
        });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) {
            throw new Error(data.error || "Ошибка импорта ключей");
        }

        showMessage(
            `Импорт завершён. Ключей: ${importState.licensesForImport.length}. Привязку делаем на странице «Лицензии».`,
            false
        );
    } catch (e) {
        console.error(e);
        showMessage("Ошибка сохранения: " + e.message, true);
    }
}

// ---------- инициализация ----------

document.addEventListener("DOMContentLoaded", () => {
    const addLicenseBtn = document.getElementById("add-license-row-btn");
    const clearLicensesBtn = document.getElementById("clear-licenses-btn");
    const licensesFileInput = document.getElementById("licenses-file-input");
    const saveBtn = document.getElementById("save-btn");

    if (addLicenseBtn) addLicenseBtn.addEventListener("click", addLicenseRow);
    if (clearLicensesBtn) clearLicensesBtn.addEventListener("click", clearLicenses);
    if (licensesFileInput) {
        licensesFileInput.addEventListener("change", (e) => {
            const file = e.target.files && e.target.files[0];
            if (file) loadLicensesFromFile(file);
            e.target.value = "";
        });
    }

    if (saveBtn) saveBtn.addEventListener("click", saveToServer);

    renderLicensesTable();
});
