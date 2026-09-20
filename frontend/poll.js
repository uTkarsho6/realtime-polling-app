// PulsePoll — Real-Time Polling Client Logic
const urlParams = new URLSearchParams(window.location.search);
const pollId = urlParams.get('id');

// DOM Elements
const questionElem = document.getElementById('pollQuestion');
const optionsListElem = document.getElementById('optionsList');
const voteBtn = document.getElementById('voteBtn');
const totalVotesElem = document.getElementById('totalVotes');
const statusPill = document.getElementById('connectionStatus');
const statusText = document.getElementById('statusText');
const shareBtn = document.getElementById('shareBtn');
const toast = document.getElementById('toast');

// State
let selectedOption = null;
let hasVoted = false;
let currentPoll = null;
let ws = null;
let reconnectTimer = null;

function showToast(message, isError = false) {
  toast.textContent = message;
  toast.style.borderColor = isError ? 'rgba(239, 68, 68, 0.4)' : 'rgba(16, 185, 129, 0.4)';
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 3000);
}

// Share Button
shareBtn.addEventListener('click', () => {
  navigator.clipboard.writeText(window.location.href).then(() => {
    showToast('Poll link copied to clipboard! 📋');
  }).catch(() => {
    showToast('Failed to copy URL', true);
  });
});

// Update Status Pill UI
function setConnectionStatus(status, text) {
  statusPill.className = `status-pill ${status}`;
  statusText.textContent = text;
}

// Render Poll Options & Live Results
function renderPoll(question, options) {
  questionElem.textContent = question;

  // Calculate total votes across all options
  let totalVotes = 0;
  for (const key in options) {
    totalVotes += options[key].count || 0;
  }
  totalVotesElem.textContent = `Total Votes: ${totalVotes.toLocaleString()}`;

  optionsListElem.innerHTML = '';

  for (const optionName in options) {
    const count = options[optionName].count || 0;
    const percentage = totalVotes > 0 ? Math.round((count / totalVotes) * 100) : 0;

    const card = document.createElement('div');
    card.className = `vote-card ${selectedOption === optionName ? 'selected' : ''}`;
    card.dataset.option = optionName;

    card.innerHTML = `
      <div class="vote-card-progress" style="width: ${percentage}%"></div>
      <div class="vote-card-content">
        <span class="option-name">
          <span>${optionName}</span>
        </span>
        <div class="option-stats">
          <span class="vote-percent">${percentage}%</span>
          <span class="vote-count-badge">${count} ${count === 1 ? 'vote' : 'votes'}</span>
        </div>
      </div>
    `;

    // Click handler to select an option
    if (!hasVoted) {
      card.addEventListener('click', () => {
        selectedOption = optionName;
        document.querySelectorAll('.vote-card').forEach(c => c.classList.remove('selected'));
        card.classList.add('selected');

        voteBtn.disabled = false;
        voteBtn.innerHTML = `<span>Submit Vote for "${optionName}"</span>`;
      });
    }

    optionsListElem.appendChild(card);
  }
}

// ------------------------------------------------------------
// BUSINESS LOGIC: 1. Fetch initial poll data via REST
// ------------------------------------------------------------
async function fetchPollData() {
  if (!pollId) {
    questionElem.textContent = 'Invalid Poll ID. Please provide a valid poll link.';
    optionsListElem.innerHTML = '';
    setConnectionStatus('offline', 'Error');
    return;
  }

  try {
    const res = await fetch(`${CONFIG.REST_API_URL}/polls/${encodeURIComponent(pollId)}`);
    if (!res.ok) {
      if (res.status === 404) {
        questionElem.textContent = 'Poll Not Found (404)';
        optionsListElem.innerHTML = '<p style="color: var(--text-muted); text-align: center; padding: 20px;">This poll does not exist or has expired.</p>';
        setConnectionStatus('offline', 'Not Found');
        return;
      }
      throw new Error(`HTTP ${res.status}`);
    }

    const data = await res.json();
    currentPoll = data;
    renderPoll(data.question, data.options);

    // Once initial REST fetch completes, open real-time WebSocket connection
    initWebSocket();

  } catch (err) {
    console.error('Error fetching poll data:', err);
    questionElem.textContent = 'Failed to load poll.';
    setConnectionStatus('offline', 'REST Error');
  }
}

// ------------------------------------------------------------
// BUSINESS LOGIC: 2. Real-Time WebSocket Connection & Broadcast Listener
// ------------------------------------------------------------
function initWebSocket() {
  if (ws) {
    ws.close();
  }

  setConnectionStatus('connecting', 'Connecting...');
  const wsUrl = `${CONFIG.WS_API_URL}?pollId=${encodeURIComponent(pollId)}`;

  try {
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      console.log('WebSocket connected:', wsUrl);
      setConnectionStatus('live', 'Live WebSocket');
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    };

    ws.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data);
        console.log('Received WebSocket broadcast update:', payload);

        // Update local options and re-render progress bars live
        if (payload.options) {
          if (!currentPoll) currentPoll = {};
          currentPoll.options = payload.options;
          renderPoll(currentPoll.question || payload.question, payload.options);
        }
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e);
      }
    };

    ws.onclose = () => {
      console.warn('WebSocket closed. Attempting reconnect in 3s...');
      setConnectionStatus('connecting', 'Reconnecting...');
      reconnectTimer = setTimeout(initWebSocket, 3000);
    };

    ws.onerror = (err) => {
      console.error('WebSocket error:', err);
      setConnectionStatus('offline', 'Disconnected');
    };

  } catch (err) {
    console.error('Failed to initialize WebSocket:', err);
    setConnectionStatus('offline', 'WS Unavailable');
  }
}

// ------------------------------------------------------------
// BUSINESS LOGIC: 3. Submit Vote via REST API
// ------------------------------------------------------------
voteBtn.addEventListener('click', async () => {
  if (!selectedOption || hasVoted) return;

  voteBtn.disabled = true;
  voteBtn.innerHTML = '<span>Recording Vote...</span>';

  try {
    const res = await fetch(`${CONFIG.REST_API_URL}/polls/${encodeURIComponent(pollId)}/vote`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ option: selectedOption })
    });

    if (!res.ok) {
      throw new Error(`Server returned HTTP ${res.status}`);
    }

    hasVoted = true;
    showToast('Vote recorded! Live results updating...');
    voteBtn.innerHTML = '<span>✓ Vote Submitted</span>';
    voteBtn.style.background = 'rgba(16, 185, 129, 0.2)';
    voteBtn.style.border = '1px solid #10b981';
    voteBtn.style.color = '#34d399';

    // Disable cursor pointer on cards
    document.querySelectorAll('.vote-card').forEach(c => c.style.cursor = 'default');

  } catch (err) {
    console.error('Vote submission failed:', err);
    showToast('Failed to record vote. Please try again.', true);
    voteBtn.disabled = false;
    voteBtn.innerHTML = `<span>Submit Vote for "${selectedOption}"</span>`;
  }
});

// Initialize on page load
fetchPollData();
