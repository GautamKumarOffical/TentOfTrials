/**
 * Tests for WebSocket reconnection backoff calculation and state transitions.
 *
 * Run with: npx tsx src/__tests__/websocket.test.ts
 */

import { calculateBackoffDelay } from '../services/websocket';

// ---------------------------------------------------------------------------
// TEST UTILITIES
// ---------------------------------------------------------------------------

let passed = 0;
let failed = 0;

function assert(condition: boolean, message: string): void {
  if (condition) {
    passed++;
    console.log(`  ✓ ${message}`);
  } else {
    failed++;
    console.error(`  ✗ ${message}`);
  }
}

function assertRange(value: number, min: number, max: number, message: string): void {
  assert(value >= min && value <= max, `${message} (got ${value}, expected [${min}, ${max}])`);
}

// ---------------------------------------------------------------------------
// BACKOFF CALCULATION TESTS
// ---------------------------------------------------------------------------

console.log('\n=== Backoff Calculation Tests ===\n');

// Test 1: First attempt should be baseDelay + random jitter
console.log('Test 1: First attempt backoff');
const delay0 = calculateBackoffDelay(0, 1000, 30000, 1000);
assertRange(delay0, 1000, 2000, 'First attempt delay is in range');

// Test 2: Second attempt should be baseDelay * 2 + random jitter
console.log('Test 2: Second attempt backoff');
const delay1 = calculateBackoffDelay(1, 1000, 30000, 1000);
assertRange(delay1, 2000, 3000, 'Second attempt delay is in range');

// Test 3: Third attempt should be baseDelay * 4 + random jitter
console.log('Test 3: Third attempt backoff');
const delay2 = calculateBackoffDelay(2, 1000, 30000, 1000);
assertRange(delay2, 4000, 5000, 'Third attempt delay is in range');

// Test 4: Delay should be capped at maxDelay
console.log('Test 4: Max delay cap');
const delayLarge = calculateBackoffDelay(20, 1000, 30000, 1000);
assertRange(delayLarge, 30000, 31000, 'Large attempt delay is capped at maxDelay');

// Test 5: Zero jitter should give exact exponential delay
console.log('Test 5: Zero jitter');
const delayNoJitter = calculateBackoffDelay(3, 1000, 30000, 0);
assert(delayNoJitter === 8000, 'Zero jitter gives exact delay');

// Test 6: Custom base delay
console.log('Test 6: Custom base delay');
const delayCustomBase = calculateBackoffDelay(0, 500, 30000, 0);
assert(delayCustomBase === 500, 'Custom base delay works');

// Test 7: Small max delay
console.log('Test 7: Small max delay');
const delaySmallMax = calculateBackoffDelay(10, 1000, 5000, 0);
assert(delaySmallMax === 5000, 'Small max delay caps correctly');

// Test 8: Exponential growth pattern
console.log('Test 8: Exponential growth pattern');
const delays = [];
for (let i = 0; i < 5; i++) {
  delays.push(calculateBackoffDelay(i, 1000, 30000, 0));
}
assert(delays[0] === 1000, 'Base delay is correct');
assert(delays[1] === 2000, 'Second delay is 2x base');
assert(delays[2] === 4000, 'Third delay is 4x base');
assert(delays[3] === 8000, 'Fourth delay is 8x base');
assert(delays[4] === 16000, 'Fifth delay is 16x base');

// Test 9: Multiple calls produce varying results due to jitter
console.log('Test 9: Jitter produces varying results');
const jitterDelays = new Set<number>();
for (let i = 0; i < 100; i++) {
  jitterDelays.add(calculateBackoffDelay(0, 1000, 30000, 1000));
}
assert(jitterDelays.size > 1, 'Jitter produces varying delays');

// Test 10: Edge case - zero base delay
console.log('Test 10: Zero base delay');
const delayZeroBase = calculateBackoffDelay(0, 0, 30000, 1000);
assertRange(delayZeroBase, 0, 1000, 'Zero base delay works');

// ---------------------------------------------------------------------------
// SUMMARY
// ---------------------------------------------------------------------------

console.log('\n=== Test Summary ===');
console.log(`Passed: ${passed}`);
console.log(`Failed: ${failed}`);
console.log(`Total: ${passed + failed}`);

if (failed > 0) {
  process.exit(1);
} else {
  console.log('\nAll tests passed!\n');
  process.exit(0);
}
