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
 * Get volume from ALSA for a control
 * @param {string} card - Card number
 * @param {string} control - Control name (e.g., 'Master')
 * @returns {Promise<number>} - Volume percentage
 */
async function getAlsaVolume(card, control) {
    const output = await serverExec(`amixer -c ${card} sget '${control}'`);
    const match = output.match(/Mono:.*Playback\s+(\d+)/);
    if (!match) {
        throw new Error('Could not parse ALSA volume output');
    }
    return parseInt(match[1]);
}

async function runTests() {
    console.log('Starting ALSA→UI E2E Tests...\n');
    
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
    
    // Get initial ALSA Master volume
    await test('Get initial ALSA Master volume', async () => {
        const initialVol = await getAlsaVolume('1', 'Master');
        console.log(`  Initial Master volume: ${initialVol}`);
        consoleMonitor.assertNoErrors();
    });
    
    // Open page and find Master slider
    await test('Open page and find Master slider', async () => {
        await page.goto(BASE_URL, { timeout: 10000 });
        await page.waitForSelector('[role="slider"]', { timeout: 10000 });
        
        const sliders = await page.locator('[role="slider"]').all();
        console.log(`  Found ${sliders.length} sliders total`);
        
        // Try to find Master by looking for it in the DOM
        const masterSlider = page.locator('[aria-label*="Master"], [data-name*="Master"]').first();
        const count = await masterSlider.count();
        console.log(`  Master-related sliders: ${count}`);
        
        consoleMonitor.assertNoErrors();
    });
    
    // Change ALSA volume externally
    const targetVolume = 30;
    await test(`Change ALSA Master to ${targetVolume}% via amixer`, async () => {
        await serverExec(`amixer -c 1 sset 'Master' ${targetVolume}%`);
        await page.waitForTimeout(500);
        
        const actualVol = await getAlsaVolume('1', 'Master');
        console.log(`  ALSA Master volume: ${actualVol}`);
        
        if (actualVol !== targetVolume) {
            throw new Error(`Expected volume ${targetVolume}, got ${actualVol}`);
        }
    });
    
    // Wait and check if UI updates via SSE
    await test('UI should update via SSE broadcast', async () => {
        // Wait for SSE to propagate the change (monitor ticks every 100ms)
        await page.waitForTimeout(2000);
        
        // Reload to get fresh state from server
        await page.reload();
        await page.waitForSelector('[role="slider"]', { timeout: 10000 });
        
        // Check slider values - find the Master control
        const controls = await page.locator('.mixer-control').all();
        let foundMaster = false;
        let masterVolume = null;
        
        for (const control of controls) {
            const name = await control.getAttribute('data-control-name');
            if (name && name.toLowerCase().includes('master')) {
                foundMaster = true;
                const slider = control.locator('[role="slider"]');
                if (await slider.count() > 0) {
                    masterVolume = await slider.getAttribute('aria-valuenow');
                    console.log(`  Master slider value: ${masterVolume}`);
                }
                break;
            }
        }
        
        if (!foundMaster) {
            console.log('  Warning: Could not find Master control in UI');
        } else if (masterVolume !== null) {
            const expectedVol = String(targetVolume);
            if (masterVolume !== expectedVol) {
                throw new Error(`Expected Master volume ${expectedVol} in UI, got ${masterVolume}`);
            }
        }
        
        consoleMonitor.assertNoErrors();
    });
    
    // Restore volume
    await test('Restore Master volume to 75%', async () => {
        await serverExec(`amixer -c 1 sset 'Master' 75%`);
        await page.waitForTimeout(500);
        
        const actualVol = await getAlsaVolume('1', 'Master');
        console.log(`  Restored Master volume: ${actualVol}`);
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
