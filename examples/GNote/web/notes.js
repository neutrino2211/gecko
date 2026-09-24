const state = {
  notes: [],
  search: "",
  editingId: null
};

const els = {
  notesGrid: document.getElementById("notesGrid"),
  status: document.getElementById("status"),
  noteCount: document.getElementById("noteCount"),
  updatedAt: document.getElementById("updatedAt"),
  searchInput: document.getElementById("searchInput"),
  editor: document.getElementById("editor"),
  overlay: document.getElementById("overlay"),
  editorTitle: document.getElementById("editorTitle"),
  noteTitle: document.getElementById("noteTitle"),
  noteBody: document.getElementById("noteBody"),
  deleteBtn: document.getElementById("deleteBtn"),
  newNoteBtn: document.getElementById("newNoteBtn"),
  saveBtn: document.getElementById("saveBtn"),
  cancelBtn: document.getElementById("cancelBtn"),
  closeEditorBtn: document.getElementById("closeEditorBtn")
};

const isNativeBound =
  typeof window.notes_list === "function" &&
  typeof window.notes_get === "function" &&
  typeof window.notes_create === "function" &&
  typeof window.notes_update === "function" &&
  typeof window.notes_delete === "function" &&
  typeof window.notes_search === "function";

const fallback = (() => {
  const key = "gnote.fallback.notes";
  const load = () => {
    try {
      return JSON.parse(localStorage.getItem(key) || "[]");
    } catch (_) {
      return [];
    }
  };
  const save = (notes) => localStorage.setItem(key, JSON.stringify(notes));
  return {
    async list() {
      return load().sort((a, b) => (b.updated_at || "").localeCompare(a.updated_at || ""));
    },
    async get(id) {
      return load().find((n) => n.id === id) || null;
    },
    async search(term) {
      const q = (term || "").toLowerCase();
      return (await this.list()).filter((n) =>
        (n.title || "").toLowerCase().includes(q) ||
        (n.body || "").toLowerCase().includes(q)
      );
    },
    async create(title, body) {
      const notes = load();
      const now = new Date().toISOString();
      const note = {
        id: Date.now(),
        title,
        body,
        created_at: now,
        updated_at: now
      };
      notes.push(note);
      save(notes);
      return { ok: true, id: note.id };
    },
    async update(id, title, body) {
      const notes = load();
      const idx = notes.findIndex((n) => n.id === id);
      if (idx >= 0) {
        notes[idx] = { ...notes[idx], title, body, updated_at: new Date().toISOString() };
        save(notes);
      }
      return { ok: true };
    },
    async delete(id) {
      save(load().filter((n) => n.id !== id));
      return { ok: true };
    }
  };
})();

const api = {
  async list(limit = 200, offset = 0) {
    return isNativeBound ? window.notes_list(limit, offset) : fallback.list(limit, offset);
  },
  async get(id) {
    return isNativeBound ? window.notes_get(id) : fallback.get(id);
  },
  async search(term, limit = 200, offset = 0) {
    return isNativeBound ? window.notes_search(term, limit, offset) : fallback.search(term, limit, offset);
  },
  async create(title, body) {
    return isNativeBound ? window.notes_create(title, body) : fallback.create(title, body);
  },
  async update(id, title, body) {
    return isNativeBound ? window.notes_update(id, title, body) : fallback.update(id, title, body);
  },
  async delete(id) {
    return isNativeBound ? window.notes_delete(id) : fallback.delete(id);
  }
};

const parseMaybeJson = (value) => {
  if (typeof value === "string") {
    try {
      return JSON.parse(value);
    } catch (_) {
      return value;
    }
  }
  return value;
};

const setStatus = (text, persist = false) => {
  els.status.textContent = text;
  if (!persist) {
    clearTimeout(setStatus._t);
    setStatus._t = setTimeout(() => {
      els.status.textContent = "";
    }, 1700);
  }
};

const nowLabel = () =>
  new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }).toLowerCase();

const toneClass = (id) => `tone-${(Math.abs(Number(id)) % 4) + 1}`;

const renderNotes = () => {
  const notes = state.notes || [];
  els.noteCount.textContent = String(notes.length);
  els.updatedAt.textContent = nowLabel();

  if (!notes.length) {
    els.notesGrid.innerHTML = `
      <div class="empty">
        No notes yet. Tap the + button to create your first one.
      </div>
    `;
    return;
  }

  els.notesGrid.innerHTML = notes.map((note) => {
    const title = (note.title || "").trim() || "Untitled";
    const body = note.body || "";
    const updated = (note.updated_at || "").replace("T", " ").slice(0, 16) || "now";
    return `
      <article class="note-card ${toneClass(note.id)}" data-id="${note.id}">
        <h3>${escapeHtml(title)}</h3>
        <p>${escapeHtml(body)}</p>
        <div class="note-meta">
          <span>${escapeHtml(updated)}</span>
          <span class="note-tag">Note</span>
        </div>
      </article>
    `;
  }).join("");
};

const escapeHtml = (text) =>
  String(text)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");

const openEditor = (mode, note = null) => {
  state.editingId = mode === "edit" && note ? Number(note.id) : null;
  els.editorTitle.textContent = mode === "edit" ? "Edit Note" : "New Note";
  els.noteTitle.value = note ? note.title || "" : "";
  els.noteBody.value = note ? note.body || "" : "";
  els.deleteBtn.style.display = mode === "edit" ? "inline-block" : "none";
  els.editor.classList.add("open");
  els.overlay.classList.add("show");
  setTimeout(() => els.noteTitle.focus(), 80);
};

const closeEditor = () => {
  state.editingId = null;
  els.editor.classList.remove("open");
  els.overlay.classList.remove("show");
};

const loadNotes = async () => {
  try {
    const q = state.search.trim();
    const raw = q ? await api.search(q, 300, 0) : await api.list(300, 0);
    const parsed = parseMaybeJson(raw);
    state.notes = Array.isArray(parsed) ? parsed : [];
    renderNotes();
  } catch (err) {
    setStatus("Failed to load notes", true);
    console.error(err);
  }
};

const saveCurrent = async () => {
  const title = els.noteTitle.value.trim();
  const body = els.noteBody.value;
  if (!title && !body.trim()) {
    setStatus("Write something before saving");
    return;
  }

  try {
    if (state.editingId == null) {
      const result = parseMaybeJson(await api.create(title, body));
      if (result && result.ok) {
        setStatus("Note created");
      } else {
        throw new Error("create failed");
      }
    } else {
      const result = parseMaybeJson(await api.update(state.editingId, title, body));
      if (result && result.ok) {
        setStatus("Note updated");
      } else {
        throw new Error("update failed");
      }
    }
    closeEditor();
    await loadNotes();
  } catch (err) {
    setStatus("Save failed", true);
    console.error(err);
  }
};

const deleteCurrent = async () => {
  if (state.editingId == null) return;
  try {
    const result = parseMaybeJson(await api.delete(state.editingId));
    if (result && result.ok) {
      setStatus("Note deleted");
      closeEditor();
      await loadNotes();
    } else {
      throw new Error("delete failed");
    }
  } catch (err) {
    setStatus("Delete failed", true);
    console.error(err);
  }
};

let searchTimer = null;
els.searchInput.addEventListener("input", () => {
  state.search = els.searchInput.value;
  clearTimeout(searchTimer);
  searchTimer = setTimeout(loadNotes, 220);
});

els.newNoteBtn.addEventListener("click", () => openEditor("create"));
els.cancelBtn.addEventListener("click", closeEditor);
els.closeEditorBtn.addEventListener("click", closeEditor);
els.overlay.addEventListener("click", closeEditor);
els.saveBtn.addEventListener("click", saveCurrent);
els.deleteBtn.addEventListener("click", deleteCurrent);

document.addEventListener("keydown", (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
    event.preventDefault();
    if (els.editor.classList.contains("open")) {
      saveCurrent();
    }
  }
  if (event.key === "Escape" && els.editor.classList.contains("open")) {
    closeEditor();
  }
});

els.notesGrid.addEventListener("click", async (event) => {
  const card = event.target.closest(".note-card");
  if (!card) return;
  const noteId = Number(card.dataset.id);
  if (!noteId) return;
  try {
    const raw = await api.get(noteId);
    const note = parseMaybeJson(raw);
    if (note) {
      openEditor("edit", note);
    }
  } catch (err) {
    setStatus("Failed to open note", true);
    console.error(err);
  }
});

loadNotes();
