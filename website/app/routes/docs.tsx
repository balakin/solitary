import type { Route } from './+types/docs';
import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import {
  DocsBody,
  DocsDescription,
  DocsPage,
  DocsTitle,
  MarkdownCopyButton,
  PageFooter,
  ViewOptionsPopover,
  type FooterProps,
} from 'fumadocs-ui/layouts/docs/page';
import { docs, getPageMarkdownUrl, source } from '@/lib/source';
import { baseOptions } from '@/lib/layout.shared';
import { appName, gitConfig, getPageImagePath } from '@/lib/shared';
import { useFumadocsLoader } from 'fumadocs-core/source/client';
import { useMDXComponents } from '@/components/mdx';
import { PageCredits } from '@/components/footer';
import { use } from 'react';

export async function loader({ params }: Route.LoaderArgs) {
  const slugs = params['*'].split('/').filter((v) => v.length > 0);
  const page = source.getPage(slugs);
  if (!page) throw new Response('Not found', { status: 404 });

  return {
    path: page.path,
    markdownUrl: getPageMarkdownUrl(page).url,
    pageTree: await source.serializePageTree(source.getPageTree()),
    imagePath: getPageImagePath(page.slugs, page.locale),
  };
}

/**
 * The end of a docs page: fumadocs' links to the previous and next pages, and
 * then who made this. The credits are children of that slot rather than a
 * sibling of the article, so they stay below the links a reader who finished
 * the page is actually looking for.
 */
function PageEnd(props: FooterProps) {
  return (
    <PageFooter {...props}>
      <PageCredits />
    </PageFooter>
  );
}

function Content({
  path,
  markdownUrl,
  imagePath,
}: {
  path: string;
  markdownUrl: string;
  imagePath: string;
}) {
  const page = docs.getPage(path);
  if (!page) throw new Error(`unknown page: ${path}`);

  // content is loaded lazily, call `page.preload()` in your loader to avoid suspending
  const { toc } = use(page.load());
  const Mdx = page.body;

  return (
    <DocsPage toc={toc} slots={{ footer: PageEnd }}>
      <title>{page.title}</title>
      <meta name="description" content={page.description} />
      <meta property="og:type" content="article" />
      <meta property="og:title" content={page.title} />
      <meta property="og:description" content={page.description} />
      <meta property="og:image" content={imagePath} />
      <meta property="og:site_name" content={appName} />
      <meta name="twitter:card" content="summary_large_image" />
      <meta name="twitter:image" content={imagePath} />
      {/* So an agent that fetched the HTML can find the plain-Markdown copy
          without knowing the URL scheme it is written under. */}
      <link rel="alternate" type="text/markdown" href={markdownUrl} />
      <DocsTitle>{page.title}</DocsTitle>
      <DocsDescription>{page.description}</DocsDescription>
      <div className="-mt-4 flex flex-row items-center gap-2 border-b pb-6">
        <MarkdownCopyButton markdownUrl={markdownUrl} />
        <ViewOptionsPopover
          markdownUrl={markdownUrl}
          githubUrl={`https://github.com/${gitConfig.user}/${gitConfig.repo}/blob/${gitConfig.branch}/content/docs/${path}`}
        />
      </div>
      <DocsBody>
        <Mdx components={useMDXComponents()} />
      </DocsBody>
    </DocsPage>
  );
}

export default function Page({ loaderData }: Route.ComponentProps) {
  const { path, pageTree, imagePath, markdownUrl } = useFumadocsLoader(loaderData);

  return (
    <DocsLayout {...baseOptions()} tree={pageTree}>
      <Content path={path} markdownUrl={markdownUrl} imagePath={imagePath} />
    </DocsLayout>
  );
}
