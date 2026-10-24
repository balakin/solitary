// Website analytics: Umami Cloud's tracker, and as little of our own around it
// as it takes. It counts pageviews and the few clicks named below; it sets no
// cookie and stores no identifier in the browser. See README.md, "Analytics".

import { repoUrl } from '@/lib/shared';

export const trackerSrc = 'https://cloud.umami.is/script.js';
export const productionHost = 'solitary.balakin.io';

/** The custom events the site sends. Pageviews need no name. */
export type AnalyticsEvent = 'install_click' | 'github_click' | 'feedback_click';

declare global {
  interface Window {
    // Absent whenever the tracker is: in development, without a website ID,
    // or when a blocker refused the script.
    umami?: { track: (event: AnalyticsEvent) => unknown };
  }
}

/**
 * The attributes of the tracker's script tag, or null when there should be no
 * tag at all: in development, and in a build that was not given a website ID —
 * a fork's or a pull request's — which would otherwise load a tracker that
 * reports to nobody.
 */
export function trackerAttributes(websiteId: string | undefined, production: boolean) {
  const id = websiteId?.trim();
  if (!production || !id) return null;

  return {
    src: trackerSrc,
    'data-website-id': id,
    // The tracker stays silent on any other hostname, so a production build
    // served from localhost or a preview reports nothing.
    'data-domains': productionHost,
    // The path is what is counted. A query string or fragment is not ours to
    // collect, and a fragment change would otherwise count a jump to a heading
    // as a pageview.
    'data-exclude-search': 'true',
    'data-exclude-hash': 'true',
    'data-do-not-track': 'true',
  } as const;
}

/**
 * Records an event for a click the tracker cannot see on its own terms. Its
 * `data-umami-event` attribute is right for a link that leaves the site, but on
 * one that stays it cancels the click and reloads the page at the new URL,
 * which turns a client-side navigation into a full page load. Internal links
 * call this from `onClick` instead; a blocked tracker makes it a no-op.
 */
export function trackEvent(event: AnalyticsEvent) {
  window.umami?.track(event);
}

/**
 * A document click listener for the links to the repository this site does not
 * render itself: fumadocs' GitHub icon in the header, the docs sidebar and the
 * mobile menu. Fumadocs builds those anchors from a fixed set of fields, so an
 * attribute cannot be put on them. A link that carries its own
 * `data-umami-event` is left to the tracker, so nothing is counted twice.
 */
export function trackRepositoryLink(event: Pick<MouseEvent, 'target'>) {
  const link = (event.target as Element | null)?.closest?.('a');
  if (link?.getAttribute('href') === repoUrl && !link.hasAttribute('data-umami-event')) {
    trackEvent('github_click');
  }
}
