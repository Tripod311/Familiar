const form = document.querySelector("#chatForm");
const input = document.querySelector("#input");
const messages = document.querySelector("#messages");

let LOCKED = false;

function addMessage(role, content) {
    const element = document.createElement("div");

    element.className = `message ${role}`;
    element.textContent = content;

    messages.appendChild(element);
    messages.scrollTop = messages.scrollHeight;

    return element;
}

async function loadHistory() {
    LOCKED = true;

    const placeholder = addMessage("thinking", "Loading history...");

    try {
        const response = await fetch("/history");

        placeholder.remove();

        if (!response.ok) {
            const errorText = await response.text();

            throw new Error(
                `${response.status} ${response.statusText}` +
                (errorText ? `\n${errorText}` : "")
            );
        }

        const history = await response.json();

        if (history) {
            for (const message of history) {
                addMessage(
                    message.role,
                    message.content
                );
            }
        }
    } catch (err) {
        placeholder.remove();

        addMessage(
            "error",
            `Failed to load history: ${err}`
        );
    } finally {
        LOCKED = false;
    }
}

async function sendMessage(message) {
    LOCKED = true;

    addMessage("user", message);
    const placeholder = addMessage("thinking", "Thinking...");

    try {
        const response = await fetch("/request", {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify({
                message
            })
        });

        placeholder.remove();

        if (!response.ok) {
            const errorText = await response.text();

            throw new Error(
                `${response.status} ${response.statusText}` +
                (errorText ? `\n${errorText}` : "")
            );
        }

        const data = await response.json();

        addMessage("assistant", data.message);
    } catch (err) {
        placeholder.remove();

        addMessage(
            "error",
            `Error: ${err}`
        );
    } finally {
        LOCKED = false;
    }
}

form.addEventListener("submit", event => {
    event.preventDefault();

    if (LOCKED) return;

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

document.addEventListener("DOMContentLoaded", loadHistory);