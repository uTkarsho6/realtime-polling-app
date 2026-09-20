// DOM Elements
const form = document.getElementById('createPollForm');
const optionsContainer = document.getElementById('optionsContainer');
const addOptionBtn = document.getElementById('addOptionBtn');
const submitBtn = document.getElementById('submitBtn');
const toast = document.getElementById('toast');

const MIN_OPTIONS = 2;
const MAX_OPTIONS = 6;

function showToast(message, isError = false) {
  toast.textContent = message;
  toast.style.borderColor = isError ? 'rgba(239, 68, 68, 0.4)' : 'rgba(16, 185, 129, 0.4)';
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 3000);
}

function updateDeleteButtons() {
  const rows = optionsContainer.querySelectorAll('.option-row');
  const deleteButtons = optionsContainer.querySelectorAll('.delete-option');
  
  deleteButtons.forEach(btn => {
    btn.disabled = rows.length <= MIN_OPTIONS;
  });

  if (rows.length >= MAX_OPTIONS) {
    addOptionBtn.style.display = 'none';
  } else {
    addOptionBtn.style.display = 'inline-flex';
  }
}

// Add dynamic option row
addOptionBtn.addEventListener('click', () => {
  const currentCount = optionsContainer.querySelectorAll('.option-row').length;
  if (currentCount >= MAX_OPTIONS) return;

  const row = document.createElement('div');
  row.className = 'option-row';
  row.innerHTML = `
    <input type="text" class="input-text option-input" placeholder="Option ${currentCount + 1}" required>
    <button type="button" class="btn-icon delete-option" aria-label="Delete option">✕</button>
  `;

  // Attach delete listener
  row.querySelector('.delete-option').addEventListener('click', () => {
    row.remove();
    updateDeleteButtons();
  });

  optionsContainer.appendChild(row);
  row.querySelector('input').focus();
  updateDeleteButtons();
});

// Attach initial delete buttons
optionsContainer.querySelectorAll('.delete-option').forEach(btn => {
  btn.addEventListener('click', (e) => {
    e.target.closest('.option-row').remove();
    updateDeleteButtons();
  });
});

// Form Submit Handler (Create Poll via REST)
form.addEventListener('submit', async (e) => {
  e.preventDefault();

  const question = document.getElementById('pollQuestion').value.trim();
  const optionInputs = optionsContainer.querySelectorAll('.option-input');
  const options = Array.from(optionInputs)
    .map(input => input.value.trim())
    .filter(val => val.length > 0);

  if (!question) {
    showToast('Please enter a poll question.', true);
    return;
  }

  if (options.length < MIN_OPTIONS) {
    showToast(`Please provide at least ${MIN_OPTIONS} options.`, true);
    return;
  }

  // Check for duplicates
  const uniqueOptions = new Set(options);
  if (uniqueOptions.size !== options.length) {
    showToast('Duplicate options are not allowed.', true);
    return;
  }

  submitBtn.disabled = true;
  submitBtn.innerHTML = '<span>Creating Poll...</span>';

  try {
    const res = await fetch(`${CONFIG.REST_API_URL}/polls`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ question, options })
    });

    if (!res.ok) {
      throw new Error(`Server returned HTTP ${res.status}`);
    }

    const data = await res.json();
    showToast('Poll created! Redirecting to live poll...');

    // Redirect to poll voting view
    setTimeout(() => {
      window.location.href = `poll.html?id=${encodeURIComponent(data.pollId)}`;
    }, 800);

  } catch (err) {
    console.error('Failed to create poll:', err);
    showToast('Failed to create poll. Please check API Gateway status.', true);
    submitBtn.disabled = false;
    submitBtn.innerHTML = '<span>Create & Launch Live Poll 🚀</span>';
  }
});
