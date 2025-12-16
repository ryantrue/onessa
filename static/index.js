// --------- утилиты ---------

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

function qs(id) {
    return document.getElementById(id);
}

// --------- состояние ---------

const listState = {
    users: [],
    licenses: [],
    filteredLicenses: [],
    filter: {
        search: "",
        user: "all" // 'all' | 'unassigned' | userId
    },
    isLoading: false
};

function setLoading(flag) {
    listState.isLoading = flag;
    const btn = qs("reload-btn");
    if (btn) {
        btn.disabled = flag;
        btn.textContent = flag ? "Обновляем..." : "Обновить";
    }
}

// --------- загрузка состояния с сервера ---------

async function loadState() {
    setLoading(true);
    showMessage("");

    try {
        const res = await fetch("/api/state", { cache: "no-store" });
        if (!res.ok) throw new Error("HTTP " + res.status);
        const data = await res.json();

        listState.users = data.users || [];
        listState.licenses = data.licenses || [];

        rebuildUserFilter();
        rebuildUsersDatalist();
        applyFilters();
        renderLicensesTable();
        updateStats();
    } catch (e) {
        console.error(e);
        showMessage("Ошибка загрузки данных: " + e.message, true);
    } finally {
        setLoading(false);
    }
}

// --------- хелперы по пользователям ---------

function getUserName(u) {
    if (!u) return "";
    if (u.name) return u.name;
    if (u.email) return u.email;
    return "user #" + u.id;
}

function findUserById(id) {
    return listState.users.find((u) => u.id === id) || null;
}

function findUserByNameCaseInsensitive(name) {
    const target = (name || "").trim().toLowerCase();
    if (!target) return null;
    return (
        listState.users.find(
            (u) => getUserName(u).trim().toLowerCase() === target
        ) || null
    );
}


function isActiveUser(u) {
    // Если бэкенд отдаёт поле active (bool/0/1) — скрываем неактивных.
    if (!u) return false;
    if (Object.prototype.hasOwnProperty.call(u, "active")) {
        return !!u.active;
    }
    return true;
}

function rebuildUsersDatalist() {
    const dl = qs("users-datalist");
    if (!dl) return;
    dl.innerHTML = "";
    listState.users.filter(isActiveUser).forEach((u) => {
        const opt = document.createElement("option");
        opt.value = getUserName(u);
        dl.appendChild(opt);
    });
}

function rebuildUserFilter() {
    const sel = qs("filter-user");
    if (!sel) return;

    const current = sel.value || "all";

    sel.innerHTML = "";
    const optAll = document.createElement("option");
    optAll.value = "all";
    optAll.textContent = "Все";
    sel.appendChild(optAll);

    const optFree = document.createElement("option");
    optFree.value = "unassigned";
    optFree.textContent = "Только свободные";
    sel.appendChild(optFree);

    listState.users.filter(isActiveUser).forEach((u) => {
        const opt = document.createElement("option");
        opt.value = String(u.id);
        opt.textContent = getUserName(u);
        sel.appendChild(opt);
    });

    sel.value = current;
    if (!sel.value) sel.value = "all";
    listState.filter.user = sel.value;
}

// --------- фильтрация ---------

function applyFilters() {
    const search = (listState.filter.search || "").trim().toLowerCase();
    const userFilter = listState.filter.user;

    listState.filteredLicenses = listState.licenses.filter((lic) => {
        // фильтр по пользователю
        if (userFilter === "unassigned") {
            if (lic.assigned_user_id && lic.assigned_user_id !== 0) {
                return false;
            }
        } else if (userFilter !== "all") {
            const userId = Number(userFilter);
            if (lic.assigned_user_id !== userId) return false;
        }

        // фильтр по поиску
        if (search) {
            const key = (lic.key || "").toLowerCase();
            const comment = (lic.comment || "").toLowerCase();
            if (!key.includes(search) && !comment.includes(search)) {
                return false;
            }
        }

        return true;
    });
}

function updateStats() {
    const el = qs("stats");
    if (!el) return;

    const total = listState.licenses.length;
    const used = listState.licenses.filter(
        (l) => l.assigned_user_id && l.assigned_user_id !== 0
    ).length;
    const free = total - used;

    el.textContent = `Всего лицензий: ${total}. Занято: ${used}. Свободно: ${free}. Показано: ${listState.filteredLicenses.length}.`;
}

// --------- отрисовка таблицы ---------

function renderLicensesTable() {
    const table = qs("licenses-table");
    if (!table) return;

    table.innerHTML = "";

    const thead = document.createElement("thead");
    const trHead = document.createElement("tr");
    ["Ключ", "Пользователь", "Комментарий", "ПК", "Действия"].forEach((text) => {
        const th = document.createElement("th");
        th.textContent = text;
        trHead.appendChild(th);
    });
    thead.appendChild(trHead);
    table.appendChild(thead);

    const tbody = document.createElement("tbody");

    if (!listState.filteredLicenses.length) {
        const trEmpty = document.createElement("tr");
        const td = document.createElement("td");
        td.colSpan = 5;
        td.textContent = "Нет данных для отображения.";
        trEmpty.appendChild(td);
        tbody.appendChild(trEmpty);
        table.appendChild(tbody);
        return;
    }

    listState.filteredLicenses.forEach((lic) => {
        const tr = document.createElement("tr");
        tr.dataset.licenseId = String(lic.id);

        // Ключ
        const tdKey = document.createElement("td");
        tdKey.textContent = lic.key || "";
        tr.appendChild(tdKey);

        // Пользователь
        const tdUser = document.createElement("td");
        const userInput = document.createElement("input");
        userInput.type = "text";
        userInput.className = "form-control form-control-sm user-input";
        const assignedUser = findUserById(lic.assigned_user_id);
        userInput.value = assignedUser ? getUserName(assignedUser) : "";
        userInput.setAttribute("list", "users-datalist");
        userInput.placeholder = "Выберите или введите пользователя";
        tdUser.appendChild(userInput);
        tr.appendChild(tdUser);

        // Комментарий
        const tdComment = document.createElement("td");
        const commentInput = document.createElement("input");
        commentInput.type = "text";
        commentInput.className = "form-control form-control-sm comment-input";
        commentInput.value = lic.comment || "";
        commentInput.placeholder = "Комментарий";
        tdComment.appendChild(commentInput);
        tr.appendChild(tdComment);

        // ПК
        const tdPC = document.createElement("td");
        const pcInput = document.createElement("input");
        pcInput.type = "text";
        pcInput.className = "form-control form-control-sm pc-input";
        pcInput.value = lic.pc || "";
        pcInput.placeholder = "ПК";
        tdPC.appendChild(pcInput);
        tr.appendChild(tdPC);

        // Действия
        const tdActions = document.createElement("td");
        const saveBtn = document.createElement("button");
        saveBtn.textContent = "Сохранить";
        saveBtn.className = "btn btn-sm btn-primary me-2 save-row-btn";
        saveBtn.dataset.licenseId = String(lic.id);

        const unassignBtn = document.createElement("button");
        unassignBtn.textContent = "Отвязать";
        unassignBtn.className = "btn btn-sm btn-outline-secondary unassign-row-btn";
        unassignBtn.dataset.licenseId = String(lic.id);
        if (!lic.assigned_user_id || lic.assigned_user_id === 0) {
            unassignBtn.disabled = true;
        }

        tdActions.appendChild(saveBtn);
        tdActions.appendChild(unassignBtn);
        tr.appendChild(tdActions);

        tbody.appendChild(tr);
    });

    table.appendChild(tbody);
}

// --------- операции сохранения ---------

async function updateLicenseMeta(licenseId, comment, pc) {
    const res = await fetch("/api/license/update", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
            license_id: licenseId,
            comment,
            pc
        })
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
        throw new Error(data.error || "Ошибка обновления лицензии");
    }
}

async function unassignLicense(licenseId) {
    const res = await fetch("/api/license/unassign", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ license_id: licenseId })
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
        throw new Error(data.error || "Ошибка отвязки лицензии");
    }
}

async function assignLicense(licenseId, userName) {
    const cleaned = (userName || "").trim();
    if (!cleaned) {
        await unassignLicense(licenseId);
        return;
    }

    // Пользователи приходят из LDAP → локально "добавлять" их нельзя.
    // Если пользователя нет в текущем списке — пробуем один раз перезагрузить состояние.
    let user = findUserByNameCaseInsensitive(cleaned);

    if (!user) {
        await loadState(); // обновим список пользователей из бэкенда (вдруг LDAP уже синкнулся)
        user = findUserByNameCaseInsensitive(cleaned);
    }

    if (!user) {
        throw new Error(
            'Пользователь "' +
                cleaned +
                '" не найден (LDAP). Обновите страницу или попросите администратора добавить доступ/включить пользователя.'
        );
    }

    const res = await fetch("/api/assign", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
            user_id: user.id,
            license_id: licenseId
        })
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
        throw new Error(data.error || "Ошибка привязки лицензии");
    }
}

async function saveRow(licenseId, tr) {
    const lic = listState.licenses.find((l) => l.id === licenseId);
    if (!lic) {
        showMessage("Лицензия не найдена в текущем состоянии.", true);
        return;
    }

    const userInput = tr.querySelector(".user-input");
    const commentInput = tr.querySelector(".comment-input");
    const pcInput = tr.querySelector(".pc-input");

    const userName = userInput ? userInput.value : "";
    const comment = commentInput ? commentInput.value : "";
    const pc = pcInput ? pcInput.value : "";

    try {
        setLoading(true);
        showMessage("Сохраняем...", false);

        await updateLicenseMeta(licenseId, comment, pc);

        if (!userName.trim()) {
            if (lic.assigned_user_id && lic.assigned_user_id !== 0) {
                await unassignLicense(licenseId);
            }
        } else {
            await assignLicense(licenseId, userName);
        }

        await loadState();
        showMessage("Изменения сохранены.", false);
    } catch (e) {
        console.error(e);
        showMessage("Ошибка сохранения: " + e.message, true);
    } finally {
        setLoading(false);
    }
}

// --------- обработчики UI ---------

function initFilters() {
    const searchInput = qs("filter-search");
    const userSelect = qs("filter-user");
    const reloadBtn = qs("reload-btn");

    if (searchInput) {
        searchInput.addEventListener("input", () => {
            listState.filter.search = searchInput.value;
            applyFilters();
            renderLicensesTable();
            updateStats();
        });
    }

    if (userSelect) {
        userSelect.addEventListener("change", () => {
            listState.filter.user = userSelect.value;
            applyFilters();
            renderLicensesTable();
            updateStats();
        });
    }

    if (reloadBtn) {
        reloadBtn.addEventListener("click", () => {
            loadState();
        });
    }
}

function initTableEvents() {
    const table = qs("licenses-table");
    if (!table) return;

    table.addEventListener("click", (e) => {
        const target = e.target;
        if (!(target instanceof HTMLElement)) return;

        if (target.classList.contains("save-row-btn")) {
            const licenseId = Number(target.dataset.licenseId);
            const tr = target.closest("tr");
            if (!tr) return;
            saveRow(licenseId, tr);
        }

        if (target.classList.contains("unassign-row-btn")) {
            const licenseId = Number(target.dataset.licenseId);
            const tr = target.closest("tr");
            if (!tr) return;

            const userInput = tr.querySelector(".user-input");
            if (userInput) userInput.value = "";

            const lic = listState.licenses.find((l) => l.id === licenseId);
            if (!lic || !lic.assigned_user_id) {
                showMessage("Лицензия уже свободна.", false);
                return;
            }

            (async () => {
                try {
                    setLoading(true);
                    showMessage("Отвязываем...", false);
                    await unassignLicense(licenseId);
                    await loadState();
                    showMessage("Лицензия отвязана.", false);
                } catch (e) {
                    console.error(e);
                    showMessage("Ошибка отвязки: " + e.message, true);
                } finally {
                    setLoading(false);
                }
            })();
        }
    });
}

// --------- старт ---------

document.addEventListener("DOMContentLoaded", () => {
    initFilters();
    initTableEvents();
    loadState();
});