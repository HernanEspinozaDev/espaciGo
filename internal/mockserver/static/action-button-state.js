export async function actionWithButtonState(buttons, work, refresh) {
    const wasDisabled = buttons.map(button => button.disabled);
    buttons.forEach(button => button.disabled = true);
    try {
        return await work();
    }
    finally {
        buttons.forEach((button, index) => button.disabled = wasDisabled[index]);
        refresh();
    }
}
