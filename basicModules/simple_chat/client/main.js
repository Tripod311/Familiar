const form = document.querySelector("#chatForm");
const input = document.querySelector("#input");
const messages = document.querySelector("#messages");

function addMessage(role, content) {
    const element = document.createElement("div");

    element.className = `message ${role}`;
    element.textContent = content;

    messages.appendChild(element);
    messages.scrollTop = messages.scrollHeight;

    return element;
}

async function sendMessage(message) {
    addMessage("user", message);

    const response = await fetch("/request", {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify({
            message
        })
    });

    if (!response.ok) {
        addMessage(
            "assistant",
            `Error: ${response.status}`
        );

        return;
    }

    const data = await response.json();

    addMessage("assistant", data.message);
}

form.addEventListener("submit", event => {
    event.preventDefault();

    const message = input.value.trim();

    if (!message) {
        return;
    }

    input.value = "";

    sendMessage(message);
});

input.addEventListener("keydown", event => {
    if (
        event.key === "Enter" &&
        !event.shiftKey
    ) {
        event.preventDefault();
        form.requestSubmit();
    }
});