import { ImageResponse } from 'takumi-js/response';
import { generate as DefaultImage } from 'fumadocs-ui/og/takumi';
import { appName, siteDescription, siteTagline } from '@/lib/shared';

export function loader() {
  return new ImageResponse(
    <DefaultImage title={siteTagline} description={siteDescription} site={appName} />,
    {
      width: 1200,
      height: 630,
      format: 'webp',
    },
  );
}
