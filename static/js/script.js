// currency selector

const currencySelect = document.getElementById("currencySelect");
const selectedOptionElement = currencySelect.options[currencySelect.selectedIndex];
const selectedOption = selectedOptionElement.innerText;

const targetElements = document.getElementsByClassName(selectedOption);
for (const el of targetElements) {
    el.classList.remove("hidden");
}

currencySelect.addEventListener("change", function () {
    const selectedOptionElement = currencySelect.options[currencySelect.selectedIndex];
    const selectedOption = selectedOptionElement.innerText;
    const prices = document.getElementsByClassName("price");
    const targetElements = document.getElementsByClassName(selectedOption);
    for (const price of prices) {
        price.classList.add("hidden");
    }
    for (const el of targetElements) {
        el.classList.remove("hidden");
    }
});
