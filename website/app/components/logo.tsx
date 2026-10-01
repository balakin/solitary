import type { SVGProps } from 'react';

/**
 * The mark: a cell's wall and the one thing inside it.
 *
 * Inline rather than an <img> of /favicon.svg so it takes `currentColor` and
 * follows the site's theme toggle; the favicon can only follow the OS. This is
 * the large-size geometry — the favicon redraws it with heavier walls set on
 * the pixel grid, which would look clumsy at anything above 32px.
 */
export function Logo(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 32 32" aria-hidden="true" {...props}>
      <polygon
        points="16,3 27.258,9.5 27.258,22.5 16,29 4.742,22.5 4.742,9.5"
        fill="none"
        stroke="currentColor"
        strokeWidth="3"
        strokeLinejoin="round"
      />
      <polygon
        points="16,10.5 20.763,13.25 20.763,18.75 16,21.5 11.237,18.75 11.237,13.25"
        fill="currentColor"
      />
    </svg>
  );
}
