import { test, expect } from './harness';

// The ask_user_question card renders one question at a time. A multi-question
// ask would otherwise stack into a tall card whose later questions scroll out
// of view. These tests drive the card through the mock event bus (window.__emit
// delivers the assistant:question event the Go backend would emit) and verify
// only one question is on screen at a time, Back/Next navigation preserves
// state, and every answer routes back through AnswerAssistantQuestion.

const THREE_QUESTIONS = {
  id: 'q-flow-1',
  questions: [
    {
      header: 'Size',
      question: 'How big should it be?',
      options: [
        { label: 'Small', description: 'compact' },
        { label: 'Large', description: 'roomy' },
      ],
    },
    {
      header: 'Colors',
      question: 'Which colors?',
      multiSelect: true,
      options: [{ label: 'Red' }, { label: 'Green' }, { label: 'Blue' }],
    },
    {
      header: 'Finish',
      question: 'What finish?',
      options: [{ label: 'Matte' }, { label: 'Glossy' }],
    },
  ],
};

async function bootAndAsk(page: any, payload: unknown) {
  await page.goto('/');
  await expect(page.locator('#editor-panel .monaco-editor').first()).toBeVisible({ timeout: 10_000 });
  // Open the assistant panel — registerEvents() runs on show(), wiring the
  // assistant:question listener into the mock event bus.
  await page.click('#assistant-btn');
  await expect(page.locator('#assistant-panel.open')).toBeVisible();
  // Deliver the ask_user_question event the backend would emit.
  await page.evaluate((p) => (window as any).__emit('assistant:question', p), payload);
  await expect(page.locator('.assistant-question-card')).toBeVisible();
}

test('multi-question card shows one question at a time with Back/Next navigation', async ({
  mockedPage: page,
}) => {
  await bootAndAsk(page, THREE_QUESTIONS);

  const card = page.locator('.assistant-question-card');
  const blocks = card.locator('.assistant-question-block');
  const progress = card.locator('.assistant-question-progress');
  const text = card.locator('.assistant-question-text');
  const next = card.locator('.assistant-question-submit');
  const back = card.locator('.assistant-question-back');

  // Q1 of 3: exactly one question block, Back hidden, Next reads "Next", and
  // only this question's options are present (no later question bleeds in).
  await expect(blocks).toHaveCount(1);
  await expect(progress).toHaveText('Question 1 of 3');
  await expect(text).toHaveText('How big should it be?');
  await expect(back).toBeHidden();
  await expect(next).toHaveText('Next');
  await expect(card.locator('.assistant-question-option')).toHaveCount(3); // Small, Large, Other

  // Validation: Next with nothing selected errors and stays on Q1.
  await next.click();
  await expect(card.locator('.assistant-question-error')).toContainText('Pick an option');
  await expect(progress).toHaveText('Question 1 of 3');

  // Pick Small → advance to Q2 (multi-select).
  await card.locator('.assistant-question-option:has-text("Small")').click();
  await next.click();
  await expect(progress).toHaveText('Question 2 of 3');
  await expect(text).toHaveText('Which colors?');
  await expect(back).toBeVisible();
  await expect(blocks).toHaveCount(1);

  // Multi-select keeps multiple selections.
  await card.locator('.assistant-question-option:has-text("Red")').click();
  await card.locator('.assistant-question-option:has-text("Blue")').click();
  await expect(card.locator('.assistant-question-option.selected')).toHaveCount(2);

  // Advance to the last question — Next becomes Submit.
  await next.click();
  await expect(progress).toHaveText('Question 3 of 3');
  await expect(next).toHaveText('Submit');

  // Back to Q2 preserves the two selections.
  await back.click();
  await expect(progress).toHaveText('Question 2 of 3');
  await expect(card.locator('.assistant-question-option.selected')).toHaveCount(2);

  // Forward, answer Q3, submit.
  await next.click();
  await card.locator('.assistant-question-option:has-text("Matte")').click();
  await next.click();

  // Card locks and every answer routes back through AnswerAssistantQuestion.
  await expect(card).toHaveClass(/answered/);
  await expect(next).toHaveText('Sent');
  const call = await page.evaluate(() =>
    (window as any).__mockCalls.find((c: any) => c.name === 'AnswerAssistantQuestion'),
  );
  expect(call.args[0]).toBe('q-flow-1');
  expect(call.args[1]).toEqual({
    'How big should it be?': 'Small',
    'Which colors?': 'Red, Blue',
    'What finish?': 'Matte',
  });
});

test('single-question card hides the progress counter and submits directly', async ({
  mockedPage: page,
}) => {
  await bootAndAsk(page, {
    id: 'q-solo',
    questions: [{ header: 'Go', question: 'Proceed?', options: [{ label: 'Yes' }, { label: 'No' }] }],
  });

  const card = page.locator('.assistant-question-card');
  await expect(card.locator('.assistant-question-progress')).toBeHidden();
  await expect(card.locator('.assistant-question-back')).toBeHidden();
  await expect(card.locator('.assistant-question-submit')).toHaveText('Submit');

  await card.locator('.assistant-question-option:has-text("Yes")').click();
  await card.locator('.assistant-question-submit').click();

  const call = await page.evaluate(() =>
    (window as any).__mockCalls.find((c: any) => c.name === 'AnswerAssistantQuestion'),
  );
  expect(call.args[1]).toEqual({ 'Proceed?': 'Yes' });
});

test('Other reveals a free-text field, validates it, and submits the typed value', async ({
  mockedPage: page,
}) => {
  await bootAndAsk(page, {
    id: 'q-other',
    questions: [{ header: 'Pick', question: 'Choose one', options: [{ label: 'Alpha' }, { label: 'Beta' }] }],
  });

  const card = page.locator('.assistant-question-card');
  const other = card.locator('.assistant-question-other');
  const submit = card.locator('.assistant-question-submit');

  await expect(other).toBeHidden();
  await card.locator('.assistant-question-option:has-text("Other")').click();
  await expect(other).toBeVisible();

  // Submitting Other with empty text errors.
  await submit.click();
  await expect(card.locator('.assistant-question-error')).toContainText('Type a custom answer');

  await other.fill('Something custom');
  await submit.click();

  const call = await page.evaluate(() =>
    (window as any).__mockCalls.find((c: any) => c.name === 'AnswerAssistantQuestion'),
  );
  expect(call.args[1]).toEqual({ 'Choose one': 'Something custom' });
  expect(call.args[2]).toEqual({ 'Choose one': 'Something custom' }); // notes carry the raw text
});
