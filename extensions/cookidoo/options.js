// options.js — persists API URL + token + token type to chrome.storage.sync.

"use strict";

const DEFAULTS = {
  api_url: "http://localhost:8080",
  token: "",
  token_type: "mobile",
};

function $(id) { return document.getElementById(id); }

async function load() {
  chrome.storage.sync.get(DEFAULTS, (items) => {
    $("api_url").value = items.api_url || DEFAULTS.api_url;
    $("token").value = items.token || "";
    const r = document.querySelector(`input[name="token_type"][value="${items.token_type || DEFAULTS.token_type}"]`);
    if (r) r.checked = true;
  });
}

function toast(msg, isError) {
  const el = $("toast");
  el.textContent = msg;
  el.classList.toggle("error", !!isError);
  el.classList.add("show");
  setTimeout(() => el.classList.remove("show"), 2000);
}

function isValidUrl(s) {
  try {
    const u = new URL(s);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch (_e) {
    return false;
  }
}

function save() {
  const api_url = $("api_url").value.trim();
  const token = $("token").value;
  const tokenTypeEl = document.querySelector('input[name="token_type"]:checked');
  const token_type = tokenTypeEl ? tokenTypeEl.value : DEFAULTS.token_type;

  if (!isValidUrl(api_url)) {
    toast("API URL must be a valid http(s) URL.", true);
    return;
  }
  if (!token) {
    toast("Token must not be empty.", true);
    return;
  }

  chrome.storage.sync.set({ api_url, token, token_type }, () => {
    if (chrome.runtime.lastError) {
      toast("Save failed: " + chrome.runtime.lastError.message, true);
    } else {
      toast("Saved.");
    }
  });
}

document.getElementById("save").addEventListener("click", save);
load();
