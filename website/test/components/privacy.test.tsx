import { screen } from '@testing-library/react';
import { expect, test } from 'vitest';
import Privacy from '@/routes/privacy';
import { renderPage } from './render';

test('the privacy policy names its controller and contact', async () => {
  renderPage(Privacy);
  expect(await screen.findByRole('heading', { level: 1, name: 'Privacy Policy' })).toBeVisible();
  expect(screen.getAllByText(/Dmitrii Balakin/).length).toBeGreaterThan(0);
  expect(screen.getAllByText(/Serbia/).length).toBeGreaterThan(0);
  expect(
    screen
      .getAllByRole('link')
      .some((link) => link.getAttribute('href') === 'mailto:privacy@balakin.io'),
  ).toBe(true);
});

test('the privacy policy carries only a last-updated date', async () => {
  const { container } = { container: document.body };
  renderPage(Privacy);
  const updated = await screen.findByText(/Last updated:/);
  const time = updated.querySelector('time');
  expect(time).toHaveAttribute('dateTime', '2026-10-24');
  expect(time).toHaveTextContent('October 24, 2026');
  expect(container).not.toHaveTextContent(/effective/i);
});
