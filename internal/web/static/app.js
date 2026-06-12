const states = [
  "PLACED",
  "PAYMENT_PENDING",
  "CONFIRMED",
  "PROCESSING",
  "PACKED",
  "DISPATCHED",
  "OUT_FOR_DELIVERY",
  "FAILED_DELIVERY",
  "DELIVERED",
  "CANCELLED",
  "PAYMENT_FAILED",
  "REFUND_ISSUED",
];

const terminalStates = new Set(["DELIVERED", "CANCELLED", "PAYMENT_FAILED", "REFUND_ISSUED"]);
let currentOrder = null;
let stream = null;

const els = {
  apiStatus: document.querySelector("#apiStatus"),
  healthButton: document.querySelector("#healthButton"),
  createForm: document.querySelector("#createForm"),
  transitionForm: document.querySelector("#transitionForm"),
  targetState: document.querySelector("#targetState"),
  triggeredBy: document.querySelector("#triggeredBy"),
  idempotencyKey: document.querySelector("#idempotencyKey"),
  metadata: document.querySelector("#metadata"),
  orderId: document.querySelector("#orderId"),
  loadButton: document.querySelector("#loadButton"),
  streamButton: document.querySelector("#streamButton"),
  newKeyButton: document.querySelector("#newKeyButton"),
  historyButton: document.querySelector("#historyButton"),
  currentState: document.querySelector("#currentState"),
  currentVersion: document.querySelector("#currentVersion"),
  updatedAt: document.querySelector("#updatedAt"),
  stateRail: document.querySelector("#stateRail"),
  historyBody: document.querySelector("#historyBody"),
  toast: document.querySelector("#toast"),
};

function init() {
  for (const state of states) {
    const option = document.createElement("option");
    option.value = state;
    option.textContent = state;
    els.targetState.append(option);
  }

  els.idempotencyKey.value = newKey();
  renderStateRail(null);
  checkHealth();

  els.healthButton.addEventListener("click", checkHealth);
  els.createForm.addEventListener("submit", createOrder);
  els.transitionForm.addEventListener("submit", applyTransition);
  els.loadButton.addEventListener("click", loadOrder);
  els.streamButton.addEventListener("click", toggleStream);
  els.newKeyButton.addEventListener("click", () => {
    els.idempotencyKey.value = newKey();
  });
  els.historyButton.addEventListener("click", loadHistory);

  document.querySelectorAll(".quick-actions button").forEach((button) => {
    button.addEventListener("click", () => quickTransition(button.dataset.state, button.dataset.cancel === "true"));
  });
}

async function checkHealth() {
  try {
    await api("/healthz");
    els.apiStatus.textContent = "API Online";
    els.apiStatus.className = "status-pill ok";
  } catch (error) {
    els.apiStatus.textContent = "API Offline";
    els.apiStatus.className = "status-pill bad";
  }
}

async function createOrder(event) {
  event.preventDefault();
  const body = {
    customerId: value("#customerId"),
    totalAmount: value("#totalAmount"),
    currency: value("#currency").toUpperCase(),
    items: [
      {
        skuId: value("#skuId"),
        quantity: Number(value("#quantity")),
        unitPrice: value("#unitPrice"),
      },
    ],
  };

  try {
    const order = await api("/api/v1/orders", {
      method: "POST",
      body: JSON.stringify(body),
    });
    setOrder(order);
    els.orderId.value = order.id;
    toast("Order created");
    await loadHistory();
  } catch (error) {
    toast(error.message);
  }
}

async function loadOrder() {
  const id = activeOrderID();
  if (!id) return toast("Order ID required");

  try {
    const order = await api(`/api/v1/orders/${encodeURIComponent(id)}`);
    setOrder(order);
    await loadHistory();
  } catch (error) {
    toast(error.message);
  }
}

async function applyTransition(event) {
  event.preventDefault();
  await transitionTo(els.targetState.value, false);
}

async function quickTransition(state, cancel) {
  els.targetState.value = state;
  await transitionTo(state, cancel);
}

async function transitionTo(state, cancel) {
  const id = activeOrderID();
  if (!id) return toast("Order ID required");

  let metadata = {};
  try {
    metadata = JSON.parse(els.metadata.value || "{}");
  } catch (error) {
    return toast("Metadata JSON is invalid");
  }

  const path = cancel
    ? `/api/v1/orders/${encodeURIComponent(id)}/cancel`
    : `/api/v1/orders/${encodeURIComponent(id)}/transition`;
  const body = cancel ? undefined : JSON.stringify({
    targetState: state,
    triggeredBy: els.triggeredBy.value,
    metadata,
  });

  try {
    const order = await api(path, {
      method: "POST",
      headers: { "Idempotency-Key": els.idempotencyKey.value || newKey() },
      body,
    });
    setOrder(order);
    els.idempotencyKey.value = newKey();
    toast(`${state} applied`);
    await loadHistory();
  } catch (error) {
    toast(error.message);
  }
}

async function loadHistory() {
  const id = activeOrderID();
  if (!id) return;

  try {
    const history = await api(`/api/v1/orders/${encodeURIComponent(id)}/history`);
    renderHistory(history);
  } catch (error) {
    toast(error.message);
  }
}

function toggleStream() {
  const id = activeOrderID();
  if (!id) return toast("Order ID required");

  if (stream) {
    stream.close();
    stream = null;
    els.streamButton.textContent = "Stream";
    toast("Stream closed");
    return;
  }

  stream = new EventSource(`/api/v1/orders/${encodeURIComponent(id)}/stream`);
  stream.addEventListener("snapshot", (event) => {
    setOrder(JSON.parse(event.data));
  });
  stream.addEventListener("order.updated", (event) => {
    setOrder(JSON.parse(event.data));
    loadHistory();
  });
  stream.onerror = () => {
    toast("Stream disconnected");
    els.streamButton.textContent = "Stream";
    stream.close();
    stream = null;
  };
  els.streamButton.textContent = "Stop";
  toast("Stream open");
}

function setOrder(order) {
  currentOrder = order;
  els.currentState.textContent = order.state;
  els.currentVersion.textContent = String(order.version);
  els.updatedAt.textContent = formatDate(order.updatedAt);
  renderStateRail(order.state);
}

function renderStateRail(activeState) {
  els.stateRail.replaceChildren();
  for (const state of states) {
    const chip = document.createElement("span");
    chip.className = "state-chip";
    if (state === activeState) chip.classList.add("active");
    if (terminalStates.has(state)) chip.classList.add("terminal");
    chip.textContent = state.replaceAll("_", " ");
    els.stateRail.append(chip);
  }
}

function renderHistory(history) {
  els.historyBody.replaceChildren();
  if (!history.length) {
    const row = document.createElement("tr");
    row.innerHTML = `<td colspan="4" class="empty-cell">No transitions</td>`;
    els.historyBody.append(row);
    return;
  }

  for (const item of history.toReversed()) {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${formatDate(item.createdAt)}</td>
      <td>${item.fromState || "-"}</td>
      <td>${item.toState}</td>
      <td>${item.triggeredBy}</td>
    `;
    els.historyBody.append(row);
  }
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
    ...options,
  });

  const text = await response.text();
  const data = text ? JSON.parse(text) : null;
  if (!response.ok) {
    throw new Error(data?.error || `Request failed: ${response.status}`);
  }
  return data;
}

function activeOrderID() {
  return els.orderId.value.trim() || currentOrder?.id || "";
}

function value(selector) {
  return document.querySelector(selector).value.trim();
}

function newKey() {
  if (crypto.randomUUID) return crypto.randomUUID();
  return `key-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function formatDate(value) {
  if (!value) return "-";
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    day: "2-digit",
    month: "short",
  }).format(new Date(value));
}

function toast(message) {
  els.toast.textContent = message;
  els.toast.classList.add("show");
  window.clearTimeout(toast.timer);
  toast.timer = window.setTimeout(() => els.toast.classList.remove("show"), 2600);
}

init();
