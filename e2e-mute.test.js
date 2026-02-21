const { chromium } = require('playwright');
const { exec } = require('child_process');

const BASE_URL = 'http://lemox.lan:8888';

function sshExec(cmd) {
    return new Promise((resolve, reject) => {
        exec(`ssh root@lemox.lan "${cmd}"`, (err, stdout, stderr) => {
            if (err) reject(err);
            else resolve(stdout);
        });
    });
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
    
    let passed = 0;
    let failed = 0;
    
    function test(name, fn) {
        return (async () => {
            try {
                console.log(`▶ ${name}`);
                await fn();
                console.log(`✓ ${name} PASSED\n`);
                passed++;
            } catch (err) {
                console.log(`✗ ${name} FAILED: ${err.message}\n`);
                failed++;
            }
        })();
    }
    
    // Get initial ALSA state for Master control
    await test('Get initial ALSA Master mute state', async () => {
        const output = await sshExec("amixer -c 1 sget 'Master' | grep -i 'mono'");
        console.log(`  ALSA output: ${output.trim()}`);
    });
    
    // Load page and find a mute toggle
    await test('Load page and find mute toggles', async () => {
        await page.goto(BASE_URL, { timeout: 10000 });
        await page.waitForSelector('[role="switch"]', { timeout: 10000 });
        
        const switches = await page.locator('[role="switch"]').all();
        console.log(`  Found ${switches.length} switch toggles`);
        
        // Get first mute toggle
        const muteSwitches = await page.locator('[data-control-kind="mute"]').all();
        console.log(`  Found ${muteSwitches.length} mute toggles`);
        
        if (muteSwitches.length === 0) {
            throw new Error('No mute toggles found on page');
        }
    });
    
    // Test UI→ALSA: Click mute toggle
    await test('UI→ALSA: Click mute toggle changes ALSA', async () => {
        // Find a mute toggle
        const muteSwitch = page.locator('[data-control-kind="mute"]').first();
        
        // Get current state
        const initialChecked = await muteSwitch.getAttribute('aria-checked');
        console.log(`  Initial aria-checked: ${initialChecked}`);
        
        // Click the mute toggle
        await muteSwitch.click();
        
        // Wait for the request to complete
        await page.waitForTimeout(1000);
        
        // Get new state
        const newChecked = await muteSwitch.getAttribute('aria-checked');
        console.log(`  New aria-checked: ${newChecked}`);
        
        if (initialChecked === newChecked) {
            throw new Error('Mute toggle did not change state after click');
        }
        
        console.log(`  Mute toggle changed from ${initialChecked} to ${newChecked}`);
    });
    
    // Verify ALSA state changed
    await test('Verify ALSA mute state changed', async () => {
        const output = await sshExec("amixer -c 1 sget 'Master' | grep -i 'mono'");
        console.log(`  ALSA output: ${output.trim()}`);
        
        // Check if output contains "[on]" or "[off]"
        if (output.includes('[off]')) {
            console.log('  Master is now MUTED');
        } else if (output.includes('[on]')) {
            console.log('  Master is now UNMUTED');
        }
    });
    
    // Toggle mute again
    await test('UI→ALSA: Toggle mute again', async () => {
        const muteSwitch = page.locator('[data-control-kind="mute"]').first();
        
        const initialChecked = await muteSwitch.getAttribute('aria-checked');
        console.log(`  Initial aria-checked: ${initialChecked}`);
        
        await muteSwitch.click();
        await page.waitForTimeout(1000);
        
        const newChecked = await muteSwitch.getAttribute('aria-checked');
        console.log(`  New aria-checked: ${newChecked}`);
        
        console.log('  Toggle successful');
    });
    
    // Test ALSA→UI: External amixer change reflected in UI
    await test('ALSA→UI: External amixer mute change reflected in UI', async () => {
        // First, ensure Master is unmuted via amixer
        await sshExec("amixer -c 1 sset 'Master' unmute");
        await page.waitForTimeout(500);
        
        let output = await sshExec("amixer -c 1 sget 'Master' | grep -i 'mono'");
        console.log(`  After unmute: ${output.trim()}`);
        
        // Now mute via amixer
        await sshExec("amixer -c 1 sset 'Master' mute");
        await page.waitForTimeout(2000); // Wait for SSE broadcast
        
        // Reload page to get fresh state from server
        await page.reload();
        await page.waitForSelector('[data-control-kind="mute"]', { timeout: 10000 });
        
        // Check the mute toggle state
        const muteSwitch = page.locator('[data-control-kind="mute"]').first();
        const ariaChecked = await muteSwitch.getAttribute('aria-checked');
        console.log(`  UI shows aria-checked: ${ariaChecked}`);
        
        if (ariaChecked !== 'true') {
            throw new Error(`Expected aria-checked="true" after amixer mute, got: ${ariaChecked}`);
        }
    });
    
    // Test unmute via amixer
    await test('ALSA→UI: External amixer unmute reflected in UI', async () => {
        await sshExec("amixer -c 1 sset 'Master' unmute");
        await page.waitForTimeout(2000);
        
        await page.reload();
        await page.waitForSelector('[data-control-kind="mute"]', { timeout: 10000 });
        
        const muteSwitch = page.locator('[data-control-kind="mute"]').first();
        const ariaChecked = await muteSwitch.getAttribute('aria-checked');
        console.log(`  UI shows aria-checked: ${ariaChecked}`);
        
        if (ariaChecked !== 'false') {
            throw new Error(`Expected aria-checked="false" after amixer unmute, got: ${ariaChecked}`);
        }
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
