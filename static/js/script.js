const currencySelect = document.getElementById("currencySelect");
const selectedOptionElement = currencySelect.options[currencySelect.selectedIndex];
const selectedOption = selectedOptionElement.innerText
const initialCurrencyDisplay = document.getElementById(selectedOption);
initialCurrencyDisplay.classList.remove('hidden');

currencySelect.addEventListener("change", function() {
	const selectedOptionElement = currencySelect.options[currencySelect.selectedIndex];
	const selectedOption = selectedOptionElement.innerText
	const prices = document.getElementsByClassName('price');
	const targetElement = document.getElementById(selectedOption); 
	for (const price of prices) {
		price.classList.add('hidden');
	}
	targetElement.classList.remove('hidden');
});
