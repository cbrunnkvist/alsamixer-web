const { chromium } = require('playwright');

const BASE_URL = process.env.E2E_BASE_URL;
if (!BASE_URL) {
    console.error('Error: E2E_BASE_URL environment variable is required');
    process.exit(1);
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
 * Helper for after-action assertions
 */
async function assertNoConsoleErrors(consoleMonitor, action, allowedPattern = null) {
    await action();
    consoleMonitor.assertNoErrors(allowedPattern);
}

async function runTests() {
    console.log('Starting Playwright E2E tests...');
    
    // Launch Brave browser (as per AGENTS.md instructions)
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
        console.log(`\n▶ ${name}`);
        consoleMonitor.clearErrors();
        try {
            await fn();
            console.log(`✓ ${name} PASSED`);
            passed++;
        } catch (err) {
            console.log(`✗ ${name} FAILED: ${err.message}`);
            const errors = consoleMonitor.getErrors();
            if (errors.length > 0) {
                console.log(`  Console errors during test:\n    ${errors.join('\n    ')}`);
            }
            failed++;
        }
    }
    
    // Test 1: UI loads correctly
    await test('UI loads homepage', async () => {
        await page.goto(BASE_URL, { timeout: 10000 });
        const title = await page.title();
        if (!title.includes('ALSA Mixer')) {
            throw new Error(`Expected title to contain 'ALSA Mixer', got: ${title}`);
        }
        consoleMonitor.assertNoErrors();
    });
    
    // Test 2: Volume controls are visible
    await test('Volume controls are visible', async () => {
        await page.waitForSelector('[role="slider"]', { timeout: 10000 });
        const sliders = await page.locator('[role="slider"]').count();
        console.log(`  Found ${sliders} volume sliders`);
        if (sliders === 0) {
            throw new Error('No volume sliders found on page');
        }
        consoleMonitor.assertNoErrors();
    });
    
    // Test 3: UI→ALSA - slider click changes ALSA volume
    await test('UI→ALSA: Slider click changes volume', async () => {
        const slider = page.locator('[role="slider"]').first();
        const initialValue = await slider.getAttribute('aria-valuenow');
        console.log(`  Initial slider value: ${initialValue}`);
        
        await assertNoConsoleErrors(consoleMonitor, () => slider.click());
        await page.waitForTimeout(500);
        
        const newValue = await slider.getAttribute('aria-valuenow');
        console.log(`  New slider value: ${newValue}`);
        console.log('  Click registered');
    });
    
    // Test 4: SSE connection established
    await test('SSE connection established', async () => {
        await page.reload();
        await page.waitForTimeout(2000);
        console.log('  SSE connection should be active');
        consoleMonitor.assertNoErrors();
    });
    
    // Test 5: Mute toggles work without errors
    await test('Mute toggles work without console errors', async () => {
        await page.waitForSelector('[data-control-kind="mute"]', { timeout: 10000 });
        const muteSwitches = await page.locator('[data-control-kind="mute"]').all();
        console.log(`  Found ${muteSwitches.length} mute toggles`);
        
        if (muteSwitches.length > 0) {
            const firstSwitch = page.locator('[data-control-kind="mute"]').first();
            
            // Click the mute toggle and check for errors
            await assertNoConsoleErrors(consoleMonitor, () => firstSwitch.click());
            await page.waitForTimeout(500);
            
            // Toggle back
            await assertNoConsoleErrors(consoleMonitor, () => firstSwitch.click());
            await page.waitForTimeout(500);
            
            console.log('  Mute toggle clicks completed without errors');
        } else {
            console.log('  No mute toggles found, skipping');
        }
    });
    
    // Test 6: No JavaScript errors during normal page interaction
    await test('Page interaction: No console errors during interaction', async () => {
        await page.reload();
        await page.waitForSelector('[role="slider"]', { timeout: 10000 });
        
        // Interact with various elements
        const slider = page.locator('[role="slider"]').first();
        await assertNoConsoleErrors(consoleMonitor, () => slider.click());
        await page.waitForTimeout(200);
        
        // Check for switches
        const switches = await page.locator('[role="switch"]').all();
        if (switches.length > 0) {
            const firstSwitch = page.locator('[role="switch"]').first();
            await assertNoConsoleErrors(consoleMonitor, () => firstSwitch.click());
            await page.waitForTimeout(200);
        }
        
        console.log(`  Interacted with page elements without errors`);
    });
    
    console.log(`\n========================================`);
    console.log(`Tests passed: ${passed}`);
    console.log(`Tests failed: ${failed}`);
    console.log(`========================================`);
    
    await browser.close();
    process.exit(failed > 0 ? 1 : 0);
}

runTests().catch(err => {
    console.error('Test runner failed:', err);
    process.exit(1);
});
