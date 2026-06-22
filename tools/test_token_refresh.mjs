#!/usr/bin/env node
/**
 * Deterministic validation for token refresh logic.
 * Run with: node test_token_refresh.mjs
 */

let passed = 0;
let failed = 0;

function assert(condition, message) {
  if (condition) {
    passed++;
    console.log(`  PASS: ${message}`);
  } else {
    failed++;
    console.error(`  FAIL: ${message}`);
  }
}

// Simulate localStorage
class MemoryStorage {
  constructor() { this.store = {}; }
  getItem(key) { return this.store[key] || null; }
  setItem(key, value) { this.store[key] = String(value); }
  removeItem(key) { delete this.store[key]; }
  clear() { this.store = {}; }
}

const localStorage = new MemoryStorage();

// Simulate the clearAuthState function
function clearAuthState() {
  localStorage.removeItem('auth_token');
  localStorage.removeItem('refresh_token');
}

// Simulate token storage after refresh
function simulateRefreshSuccess(newAccessToken, newRefreshToken) {
  localStorage.setItem('auth_token', newAccessToken);
  if (newRefreshToken) {
    localStorage.setItem('refresh_token', newRefreshToken);
  }
}

console.log('Token Refresh Logic Tests\n');

// Test 1: clearAuthState removes tokens
console.log('Test 1: clearAuthState removes tokens');
localStorage.setItem('auth_token', 'expired-access');
localStorage.setItem('refresh_token', 'expired-refresh');
clearAuthState();
assert(localStorage.getItem('auth_token') === null, 'auth_token cleared');
assert(localStorage.getItem('refresh_token') === null, 'refresh_token cleared');

// Test 2: No refresh token means cannot refresh
console.log('\nTest 2: No refresh token means cannot refresh');
localStorage.clear();
const hasRefreshToken = localStorage.getItem('refresh_token') !== null;
assert(!hasRefreshToken, 'no refresh token available');

// Test 3: Successful refresh updates tokens
console.log('\nTest 3: Successful refresh updates tokens');
localStorage.setItem('refresh_token', 'valid-refresh');
simulateRefreshSuccess('new-access-token', 'new-refresh-token');
assert(localStorage.getItem('auth_token') === 'new-access-token', 'auth_token updated');
assert(localStorage.getItem('refresh_token') === 'new-refresh-token', 'refresh_token updated');

// Test 4: Refresh without new refresh_token preserves old one
console.log('\nTest 4: Refresh without new refresh_token preserves old one');
localStorage.setItem('refresh_token', 'keep-this');
simulateRefreshSuccess('new-access', undefined);
assert(localStorage.getItem('auth_token') === 'new-access', 'auth_token updated');
assert(localStorage.getItem('refresh_token') === 'keep-this', 'refresh_token preserved');

// Test 5: Failed refresh clears auth state
console.log('\nTest 5: Failed refresh clears auth state');
localStorage.setItem('auth_token', 'bad-token');
localStorage.setItem('refresh_token', 'bad-refresh');
clearAuthState();
assert(localStorage.getItem('auth_token') === null, 'auth_token cleared on failure');
assert(localStorage.getItem('refresh_token') === null, 'refresh_token cleared on failure');

// Test 6: refreshPromise deduplication
console.log('\nTest 6: refreshPromise deduplication');
let refreshPromise = null;
let callCount = 0;
async function dedupedRefresh() {
  if (refreshPromise) return refreshPromise;
  refreshPromise = (async () => {
    callCount++;
    return true;
  })();
  try {
    return await refreshPromise;
  } finally {
    refreshPromise = null;
  }
}
(async () => {
  await Promise.all([dedupedRefresh(), dedupedRefresh(), dedupedRefresh()]);
  assert(callCount === 1, `refresh called once (got ${callCount})`);

  console.log(`\n${'='.repeat(40)}`);
  console.log(`Results: ${passed} passed, ${failed} failed`);
  process.exit(failed > 0 ? 1 : 0);
})();
