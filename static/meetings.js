// meetings.js — загрузка и отрисовка встреч (Bootstrap-верстка)

async function reloadMeetings() {
    await loadMeetings(true);
}

// ---- Вспомогательные функции ----

function parseDate(str) {
    if (!str) return null;
    const normalized = String(str).replace(" ", "T");
    const d = new Date(normalized);
    return isNaN(d.getTime()) ? null : d;
}

function formatDateTimeRu(d) {
    if (!d) return "";
    return d.toLocaleString("ru-RU", {
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit"
    });
}

function formatTimeRu(d) {
    if (!d) return "";
    return d.toLocaleTimeString("ru-RU", {
        hour: "2-digit",
        minute: "2-digit"
    });
}

function meetingProgressPercent(start, end) {
    if (!start || !end) return 0;
    const now = new Date();
    const s = start.getTime();
    const e = end.getTime();
    const n = now.getTime();

    if (e <= s) return 100;
    if (n <= s) return 0;
    if (n >= e) return 100;

    const total = e - s;
    const passed = n - s;
    const p = (passed / total) * 100;
    return Math.max(0, Math.min(100, Math.round(p)));
}

function meetingState(start, end, isCanceled) {
    if (isCanceled) return "Отменено";
    if (!start || !end) return "Неизвестно";

    const now = new Date();
    if (now < start) return "Не началась";
    if (now > end) return "Завершена";
    return "Идёт";
}

function htmlEscape(str) {
    if (str == null) return "";
    return String(str)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/\"/g, "&quot;")
        .replace(/'/g, "&#39;");
}

function isBirthdayMeeting(subject) {
    const text = String(subject || "").toLowerCase();
    return (
        text.includes("день рождения") ||
        text.includes("др ") ||
        text.includes("др.") ||
        text.includes("birthday")
    );
}

function showError(msg) {
    const el = document.getElementById("globalError");
    if (!el) return;
    if (!msg) {
        el.classList.add("d-none");
        el.textContent = "";
        return;
    }
    el.textContent = msg;
    el.classList.remove("d-none");
}

function badge(html, cls) {
    return `<span class="badge ${cls}">${html}</span>`;
}

function stateBadge(stateText) {
    if (stateText === "Идёт") return badge("Идёт", "text-bg-success");
    if (stateText === "Отменено") return badge("Отменено", "text-bg-danger");
    // "Не началась" / "Завершена" / "Неизвестно"
    return badge(htmlEscape(stateText), "text-bg-secondary");
}

// ---- Основная загрузка ----

async function loadMeetings(force) {
    const listEl = document.getElementById("list");
    const emptyEl = document.getElementById("emptyState");
    const expEl = document.getElementById("exportedAt");
    const statusLine = document.getElementById("statusLine");
    const statusCount = document.getElementById("statusCount");

    if (!listEl || !emptyEl || !expEl || !statusLine || !statusCount) {
        console.error("meetings.js: не найдены необходимые элементы DOM");
        return;
    }

    if (!force) {
        expEl.innerHTML = "<small>Загрузка данных…</small>";
    }

    showError("");
    emptyEl.classList.add("d-none");
    listEl.innerHTML = "";
    statusLine.classList.add("d-none");

    let resp;
    try {
        resp = await fetch("/api/meetings", { cache: "no-store" });
    } catch (e) {
        showError("Не удалось обратиться к /api/meetings: " + e);
        return;
    }

    if (!resp.ok) {
        let msg = "Ошибка HTTP " + resp.status;
        try {
            const errBody = await resp.json();
            if (errBody && errBody.error) msg += ": " + errBody.error;
        } catch {
            // ignore
        }
        showError(msg);
        return;
    }

    let data;
    try {
        data = await resp.json();
    } catch (e) {
        showError("Не удалось прочитать JSON ответа: " + e);
        return;
    }

    const items = Array.isArray(data.items) ? data.items.slice() : [];

    // сортируем по началу
    items.sort((a, b) => {
        const da = parseDate(a.start);
        const db = parseDate(b.start);
        const ta = da ? da.getTime() : 0;
        const tb = db ? db.getTime() : 0;
        return ta - tb;
    });

    // "Последнее обновление"
    if (data.exported_at) {
        const exportedAtDate = parseDate(data.exported_at);
        const txt = exportedAtDate
            ? "Последнее обновление: " + formatDateTimeRu(exportedAtDate)
            : "Последнее обновление (сырое значение): " + htmlEscape(data.exported_at);
        expEl.innerHTML = "<small>" + txt + "</small>";
    } else {
        expEl.innerHTML = "<small>Выгрузка из Outlook пока не выполнялась</small>";
    }

    if (items.length === 0) {
        emptyEl.classList.remove("d-none");
        return;
    }

    statusCount.textContent = "Всего встреч: " + items.length;
    statusLine.classList.remove("d-none");

    for (const item of items) {
        const start = parseDate(item.start);
        const end = parseDate(item.end);
        const isCanceled = !!item.is_canceled;
        const isRecurring = !!item.is_recurring;

        const pct = meetingProgressPercent(start, end);
        const stateText = meetingState(start, end, isCanceled);

        const title = htmlEscape(item.subject || "(без темы)");
        const loc = htmlEscape(item.location || "");
        const link = item.link ? String(item.link).trim() : "";
        const participants = htmlEscape(item.participants || "");

        const startStr = start ? formatDateTimeRu(start) : htmlEscape(item.start || "");
        const endStr = end ? formatDateTimeRu(end) : htmlEscape(item.end || "");
        const timeRange =
            start && end
                ? htmlEscape(formatDateTimeRu(start) + " — " + formatTimeRu(end))
                : htmlEscape(item.start || "");

        const isBirthday = isBirthdayMeeting(item.subject);

        const card = document.createElement("div");
        card.className = "card";
        if (isCanceled) card.classList.add("border-danger");
        if (isBirthday) card.classList.add("border-warning", "bg-warning-subtle");

        const badges = [
            stateBadge(stateText),
            badge(timeRange, "text-bg-light border text-dark"),
            badge("Повтор: " + (isRecurring ? "Да" : "Нет"), "text-bg-secondary")
        ];
        if (loc) badges.push(badge("Место: " + loc, "text-bg-light border text-dark"));
        if (isBirthday) badges.push(badge("🎉 День рождения", "text-bg-warning"));

        const progressHtml =
            start && end && !isCanceled
                ? `
          <div class="mt-2">
            <div class="progress" style="height: 6px;">
              <div class="progress-bar" role="progressbar" style="width: ${pct}%;" aria-valuenow="${pct}" aria-valuemin="0" aria-valuemax="100"></div>
            </div>
            <div class="text-muted small mt-1">Прогресс: ${pct}%</div>
          </div>
        `
                : "";

        const participantsHtml = participants
            ? `<div class="mt-2"><div class="text-muted small">Участники</div><div>${participants}</div></div>`
            : "";

        const linkHtml = link
            ? `<a class="btn btn-sm btn-primary mt-3" href="${htmlEscape(
                link
            )}" target="_blank" rel="noopener noreferrer">Перейти по ссылке</a>`
            : "";

        card.innerHTML = `
      <div class="card-body">
        <div class="d-flex justify-content-between gap-2 align-items-start">
          <h2 class="h5 mb-1">${title}</h2>
        </div>

        <div class="d-flex flex-wrap gap-2 mt-2">
          ${badges.join("\n")}
        </div>

        ${progressHtml}

        <div class="text-muted small mt-2">Начало: ${startStr}<br/>Конец: ${endStr}</div>
        ${participantsHtml}
        ${linkHtml}
      </div>
    `;

        listEl.appendChild(card);
    }
}

// Стартовая загрузка
document.addEventListener("DOMContentLoaded", () => {
    loadMeetings(false);
});
