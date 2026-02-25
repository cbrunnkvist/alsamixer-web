const { chromium } = require('playwright');

const BASE_URL = process.env.E2E_BASE_URL;
const SERVER_CMD_PREFIX = process.env.E2E_SERVER_CMD_PREFIX || '';

/**
 * E2E test for slow drag behavior - verifies that SSE updates don't
 * override the slider during active dragging.
 */
async function runTests() {
    console.log('Starting Slow Drag E2E Tests...\n');
    
    const browser = await chromium.launch({
        executablePath: '/Applications/Brave Browser.app/Contents/MacOS/Brave Browser',
        headless: false,
        args: ['--no-sandbox']
    });
    
    const context = await browser.newContext();
    const page = await context.newPage();
    
    let passed = 0;
    let failed = 0;
    
    async function test(name, fn) {
        console.log(`\n▶ ${name}`);
        try {
            await fn();
            console.log(`✓ ${name} PASSED`);
            passed++;
        } catch (err) {
            console.log(`✗ ${name} FAILED: ${err.message}`);
            failed++;
        }
    }
    
    // Helper to run amixer command
    async function amixer(args) {
        const cmd = SERVER_CMD_PREFIX ? `${SERVER_CMD_PREFIX} "amixer ${args}"` : `amixer ${args}`;
        const { exec } = require('child_process');
        return new Promise((resolve, reject) => {
            exec(cmd, (err, stdout, stderr) => {
                if (err) reject(err);
                else resolve(stdout);
            });
        });
    }
    
    // Get ALSA Master volume
    async function getMasterVolume() {
        const output = await amixer('sget Master');
        const match = output.match(/(?:Front Left|Mono): Playback \d+ \[(\d+)%\]/);
        return match ? parseInt(match[1], 10) : null;
    }
    
    // Get slider value
    async function getSliderValue(slider) {
        return parseInt(await slider.getAttribute('aria-valuenow'), 10);
    }
    
    // Test 1: Load page and find Master slider
    await test('Load page and find Master slider', async () => {
        await page.goto(BASE_URL, { timeout: 10000 });
        await page.waitForSelector('[role="slider"]', { timeout: 10000 });
        
        // Find Master slider by looking for control with data-control-name containing "Master"
        const masterControl = page.locator('.mixer-control[data-control-name*="Master"]').first();
        const slider = masterControl.locator('[role="slider"]');
        
        const count = await slider.count();
        if (count === 0) {
            throw new Error('Master slider not found');
        }
        
        console.log(`  Found Master slider`);
    });
    
    // Test 2: Set initial volume to 75%
    await test('Set initial volume to 75%', async () => {
        await amixer('sset Master 75%');
        await page.waitForTimeout(500);
        
        // Reload to get fresh state
        await page.reload();
        await page.waitForSelector('[role="slider"]', { timeout: 10000 });
        
        const masterControl = page.locator('.mixer-control[data-control-name*="Master"]').first();
        const slider = masterControl.locator('[role="slider"]');
        const value = await getSliderValue(slider);
        
        console.log(`  Initial slider value: ${value}`);
        
        // Verify ALSA also has the value
        const alsaValue = await getMasterVolume();
        console.log(`  ALSA Master value: ${alsaValue}`);
        
        if (Math.abs(value - 75) > 10) {
            throw new Error(`Expected volume around 75%, got ${value}%`);
        }
    });
    
    // Test 3: Slow drag - should NOT have jittery behavior
    await test('Slow drag: volume should decrease smoothly without jitter', async () => {
        const masterControl = page.locator('.mixer-control[data-control-name*="Master"]').first();
        const slider = masterControl.locator('[role="slider"]');
        const sliderBox = await slider.boundingBox();
        
        if (!sliderBox) {
            throw new Error('Could not get slider bounding box');
        }
        
        const startValue = await getSliderValue(slider);
        console.log(`  Starting value: ${startValue}%`);
        
        // Enable debug logging in browser
        await page.evaluate(() => {
            window.app = window.app || {};
            window.app.debugLogging = true;
        });
        
        // Get initial ALSA value
        const initialALSA = await getMasterVolume();
        
        // Perform slow drag from center to left (decreasing volume)
        const startX = sliderBox.x + sliderBox.width * 0.7;
        const endX = sliderBox.x + sliderBox.width * 0.3;
        const steps = 10;
        const stepDelay = 100; // 100ms between steps
        
        console.log(`  Dragging from ${startX} to ${endX} in ${steps} steps...`);
        
        await page.mouse.move(startX, sliderBox.y + sliderBox.height / 2);
        await page.mouse.down();
        
        let lastValue = startValue;
        let jitterDetected = false;
        
        for (let i = 0; i < steps; i++) {
            const x = startX + (endX - startX) * (i / steps);
            await page.mouse.move(x, sliderBox.y + sliderBox.height / 2);
            await page.waitForTimeout(stepDelay);
            
            const currentValue = await getSliderValue(slider);
            console.log(`    Step ${i + 1}: slider=${currentValue}%`);
            
            // Check for jitter: value should generally decrease, not jump up significantly
            if (currentValue > lastValue + 5) {
                console.log(`    ⚠ Jitter detected: ${lastValue}% -> ${currentValue}% (increase!)`);
                jitterDetected = true;
            }
            
            lastValue = currentValue;
        }
        
        await page.mouse.up();
        await page.waitForTimeout(500);
        
        const finalValue = await getSliderValue(slider);
        console.log(`  Final slider value: ${finalValue}%`);
        
        const finalALSA = await getMasterVolume();
        console.log(`  Final ALSA value: ${finalALSA}%`);
        
        if (jitterDetected) {
            throw new Error('Jitter detected: slider value jumped up during drag');
        }
        
        // Volume should have decreased
        if (finalValue >= startValue) {
            throw new Error(`Expected volume to decrease from ${startValue}%, but got ${finalValue}%`);
        }
    });
    
    // Test 4: After drag ends, SSE should sync correctly
    await test('After drag: SSE should sync final value', async () => {
        await page.waitForTimeout(1000);
        
        const masterControl = page.locator('.mixer-control[data-control-name*="Master"]').first();
        const slider = masterControl.locator('[role="slider"]');
        const sliderValue = await getSliderValue(slider);
        
        const alsaValue = await getMasterVolume();
        
        console.log(`  Slider value: ${sliderValue}%`);
        console.log(`  ALSA value: ${alsaValue}%`);
        
        // Values should be close (within quantization tolerance)
        if (Math.abs(sliderValue - alsaValue) > 10) {
            throw new Error(`Slider (${sliderValue}%) doesn't match ALSA (${alsaValue}%)`);
        }
    });
    
    console.log(`\n========================================`);
    console.log(`Tests passed: ${passed}`);
    console.log(`Tests failed: ${failed}`);
    console.log(`========================================`);
    
    await browser.close();
    process.exit(failed > 0 ? 1 : 0);
}

runTests().catch(err => {
    console.error('Test error:', err);
    process.exit(1);
});
