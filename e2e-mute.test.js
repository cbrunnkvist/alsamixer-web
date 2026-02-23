const { chromium } = require('playwright');
const { exec } = require('child_process');

const BASE_URL = process.env.E2E_BASE_URL;
if (!BASE_URL) {
    console.error('Error: E2E_BASE_URL environment variable is required');
    process.exit(1);
}

const SERVER_CMD_PREFIX = process.env.E2E_SERVER_CMD_PREFIX || '';

function serverExec(cmd) {
    return new Promise((resolve, reject) => {
        const fullCmd = SERVER_CMD_PREFIX ? `${SERVER_CMD_PREFIX} "${cmd}"` : cmd;
        exec(fullCmd, (err, stdout, stderr) => {
            if (err) reject(err);
            else resolve(stdout);
        });
    });
}

/**
 * Helper class to capture and assert on browser console messages
 */
class ConsoleMonitor {
    constructor(page) {
        this.page = page;
        this.messages = [];
        this.errors = [];
        
        page.on('console', msg => {
            this.messages.push({
                type: msg.type(),
                text: msg.text(),
                timestamp: Date.now()
            });
            // Only treat actual console.error() calls as errors, not network failures
            if (msg.type() === 'error') {
                this.errors.push(msg.text());
            }
        });
        
        page.on('pageerror', err => {
            this.errors.push(`PageError: ${err.message}`);
        });
    }
    
    /**
     * Assert that no console errors occurred since the last clear.
     * @param {RegExp} allowedPattern - Optional pattern of errors to allow
     * @throws {Error} if unexpected console errors were found
     */
    assertNoErrors(allowedPattern = null) {
        // Known harmless errors to ignore
        const knownHarmlessPatterns = [
            /Failed to load resource.*404 \(Not Found\)$/,  // Network 404 - unknown source but harmless
            /^Event$/,  // HTMX logs Event object to console (not an error)
        ];
        
        const unexpectedErrors = this.errors.filter(err => {
            // Check against allowed pattern
            if (allowedPattern && allowedPattern.test(err)) {
                return false;
            }
            // Check against known harmless patterns
            for (const pattern of knownHarmlessPatterns) {
                if (pattern.test(err)) {
                    return false;
                }
            }
            return true;
        });
        
        if (unexpectedErrors.length > 0) {
            throw new Error(`Unexpected console errors:\n  ${unexpectedErrors.join('\n  ')}`);
        }
    }
    
    /**
     * Clear recorded errors (call before an action to isolate errors from that action)
     */
    clearErrors() {
        this.errors = [];
        this.messages = [];
    }
    
    /**
     * Get all error messages
     */
    getErrors() {
        return [...this.errors];
    }
}

/**
 * Get mute state from ALSA for a control
 * @param {string} card - Card number
 * @param {string} control - Control name (e.g., 'Master')
 * @returns {Promise<boolean>} - true if muted, false if unmuted
 */
async function getAlsaMuteState(card, control) {
    const output = await serverExec(`amixer -c ${card} sget '${control}'`);
    // Look for mono playback line which contains [on] or [off]
    const monoMatch = output.match(/Mono:.*\[(on|off)\]/i);
    if (monoMatch) {
        return monoMatch[1].toLowerCase() === 'off';
    }
    // Fallback: look for any [off] or [on] in the output
    if (output.includes('[off]')) {
        return true;
    }
    return false;
}

/**
 * Assert that UI mute state matches ALSA mute state
 * @param {object} page - Playwright page
 * @param {ConsoleMonitor} consoleMonitor - Console monitor instance
 * @param {string} expectedAlsaMuted - Expected ALSA mute state (true/false)
 * @param {string} card - Card number for ALSA check
 * @param {string} control - Control name for ALSA check
 */
async function assertMuteStateSync(page, consoleMonitor, expectedAlsaMuted, card = '1', control = 'Master') {
    // Check UI state - find the specific control's mute toggle
    const controlName = control === 'Master' ? 'Master Playback Volume' : control;
    const muteSwitch = page.locator(`[data-control-name="${controlName}"][data-control-kind="mute"]`).first();
    const uiMuted = await muteSwitch.getAttribute('aria-checked');
    const uiMutedBool = uiMuted === 'true';
    
    // Check ALSA state
    const alsaMuted = await getAlsaMuteState(card, control);
    
    console.log(`  UI mute state: ${uiMuted} (muted=${uiMutedBool})`);
    console.log(`  ALSA mute state: muted=${alsaMuted}`);
    
    if (uiMutedBool !== alsaMuted) {
        throw new Error(`UI and ALSA state mismatch: UI shows muted=${uiMutedBool}, ALSA shows muted=${alsaMuted}`);
    }
    
    if (alsaMuted !== expectedAlsaMuted) {
        throw new Error(`Unexpected ALSA state: expected muted=${expectedAlsaMuted}, got muted=${alsaMuted}`);
    }
    
    // Check for console errors
    consoleMonitor.assertNoErrors();
}

async function runTests() {
    console.log('Starting Mute Toggle E2E Tests...\n');
    
    const browser = await chromium.launch({
        executablePath: '/Applications/Brave Browser.app/Contents/MacOS/Brave Browser',
        headless: false,
        args: ['--no-sandbox']
    });
    
    const context = await browser.newContext();
    const page = await context.newPage();
    const consoleMonitor = new ConsoleMonitor(page);
    
    let passed = 0;
    let failed = 0;
    
    async function test(name, fn) {
        console.log(`▶ ${name}`);
        consoleMonitor.clearErrors();
        try {
            await fn();
            console.log(`✓ ${name} PASSED\n`);
            passed++;
        } catch (err) {
            console.log(`✗ ${name} FAILED: ${err.message}\n`);
            const errors = consoleMonitor.getErrors();
            if (errors.length > 0) {
                console.log(`  Console errors during test:\n    ${errors.join('\n    ')}\n`);
            }
            failed++;
        }
    }
    
    // Helper for after-action assertions
    async function assertNoConsoleErrors(action, allowedPattern = null) {
        await action();
        consoleMonitor.assertNoErrors(allowedPattern);
    }
    
    // Test 1: Load page and find mute toggles
    await test('Load page and find mute toggles', async () => {
        await page.goto(BASE_URL, { timeout: 10000 });
        await page.waitForSelector('[role="switch"]', { timeout: 10000 });
        
        const switches = await page.locator('[role="switch"]').all();
        console.log(`  Found ${switches.length} switch toggles`);
        
        const muteSwitches = await page.locator('[data-control-kind="mute"]').all();
        console.log(`  Found ${muteSwitches.length} mute toggles`);
        
        if (muteSwitches.length === 0) {
            throw new Error('No mute toggles found on page');
        }
        
        consoleMonitor.assertNoErrors();
    });
    
    // Test 2: Ensure Master is initially unmuted via amixer
    await test('Setup: Ensure Master is unmuted via amixer', async () => {
        await serverExec("amixer -c 1 sset 'Master' unmute");
        await page.waitForTimeout(500);
        
        const alsaMuted = await getAlsaMuteState('1', 'Master');
        if (alsaMuted) {
            throw new Error('Master should be unmuted but ALSA reports muted');
        }
        console.log('  Master is unmuted');
    });
    
    // Test 3: Reload page and verify UI sync with ALSA
    await test('UI sync: Reload page shows correct unmuted state', async () => {
        await page.reload();
        await page.waitForSelector('[data-control-kind="mute"]', { timeout: 10000 });
        
        await assertMuteStateSync(page, consoleMonitor, false);
    });
    
    // Test 4: UI→ALSA - Click mute toggle should mute
    await test('UI→ALSA: Click mute toggle mutes the control', async () => {
        // Find the Master mute toggle specifically
        const muteSwitch = page.locator('[data-control-name="Master Playback Volume"][data-control-kind="mute"]').first();
        
        const initialChecked = await muteSwitch.getAttribute('aria-checked');
        console.log(`  Initial aria-checked: ${initialChecked}`);
        
        await assertNoConsoleErrors(() => muteSwitch.click());
        await page.waitForTimeout(500);
        
        // Verify UI and ALSA are both muted now
        await assertMuteStateSync(page, consoleMonitor, true, '1', 'Master');
    });
    
    // Test 5: UI→ALSA - Click mute toggle again should unmute
    await test('UI→ALSA: Click mute toggle unmutes the control', async () => {
        const muteSwitch = page.locator('[data-control-name="Master Playback Volume"][data-control-kind="mute"]').first();
        
        await assertNoConsoleErrors(() => muteSwitch.click());
        await page.waitForTimeout(500);
        
        // Verify UI and ALSA are both unmuted now
        await assertMuteStateSync(page, consoleMonitor, false, '1', 'Master');
    });
    
    // Test 6: ALSA→UI - External amixer mute reflected in UI
    await test('ALSA→UI: External amixer mute reflected in UI', async () => {
        // Mute via amixer
        await serverExec("amixer -c 1 sset 'Master' mute");
        await page.waitForTimeout(1500); // Wait for SSE broadcast
        
        // Reload page to get fresh state from server
        await page.reload();
        await page.waitForSelector('[data-control-kind="mute"]', { timeout: 10000 });
        
        // Verify UI shows muted
        await assertMuteStateSync(page, consoleMonitor, true);
    });
    
    // Test 7: ALSA→UI - External amixer unmute reflected in UI
    await test('ALSA→UI: External amixer unmute reflected in UI', async () => {
        // Unmute via amixer
        await serverExec("amixer -c 1 sset 'Master' unmute");
        await page.waitForTimeout(1500); // Wait for SSE broadcast
        
        // Reload page to get fresh state from server
        await page.reload();
        await page.waitForSelector('[data-control-kind="mute"]', { timeout: 10000 });
        
        // Verify UI shows unmuted
        await assertMuteStateSync(page, consoleMonitor, false);
    });
    
    // Test 8: Rapid toggle test
    await test('Stress test: Rapid mute toggle', async () => {
        const muteSwitch = page.locator('[data-control-name="Master Playback Volume"][data-control-kind="mute"]').first();
        
        // Toggle 3 times rapidly
        for (let i = 0; i < 3; i++) {
            await muteSwitch.click();
            await page.waitForTimeout(300);
        }
        
        await page.waitForTimeout(500);
        
        // Final state should be muted (3 toggles from unmuted)
        await assertMuteStateSync(page, consoleMonitor, true, '1', 'Master');
        
        // Cleanup: unmute
        await serverExec("amixer -c 1 sset 'Master' unmute");
    });
    
    // Test 9: No JavaScript errors on page interactions
    await test('Page interaction: No console errors during slider interaction', async () => {
        await page.reload();
        await page.waitForSelector('[role="slider"]', { timeout: 10000 });
        
        const slider = page.locator('[role="slider"]').first();
        
        // Interact with slider
        await assertNoConsoleErrors(() => slider.click());
        await page.waitForTimeout(200);
        
        // Check mute toggle still works
        const muteSwitch = page.locator('[data-control-name="Master Playback Volume"][data-control-kind="mute"]').first();
        await assertNoConsoleErrors(() => muteSwitch.click());
        await page.waitForTimeout(300);
        
        // Restore state
        await muteSwitch.click();
        await page.waitForTimeout(300);
    });
    
    console.log('========================================');
    console.log(`Tests passed: ${passed}`);
    console.log(`Tests failed: ${failed}`);
    console.log('========================================');
    
    await browser.close();
    process.exit(failed > 0 ? 1 : 0);
}

runTests().catch(err => {
    console.error('Test runner failed:', err);
    process.exit(1);
});
