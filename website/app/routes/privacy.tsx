import type { Route } from './+types/privacy';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { Link } from 'react-router';
import { homeOptions } from '@/lib/layout.shared';
import { Footer } from '@/components/footer';
import { appName, licenseUrl, repoUrl, siteImageMeta } from '@/lib/shared';

const title = `Privacy Policy | ${appName}`;
const description =
  'How the Solitary website handles personal data: cookie-free Umami analytics stored in the EU, GitHub Pages hosting, and no telemetry in the Solitary application.';

// A substantive change to the text below moves this date, and nothing else is
// dated: there is no separate effective date to keep in step with it.
const lastUpdated = 'October 24, 2026';
const lastUpdatedIso = '2026-10-24';

const contactEmail = 'privacy@balakin.io';

export function meta(_args: Route.MetaArgs) {
  return [
    { title },
    { name: 'description', content: description },
    { property: 'og:type', content: 'website' },
    { property: 'og:title', content: title },
    { property: 'og:description', content: description },
    { property: 'og:site_name', content: appName },
    ...siteImageMeta(),
  ];
}

function Email() {
  return <a href={`mailto:${contactEmail}`}>{contactEmail}</a>;
}

function External({ href, children }: { href: string; children?: React.ReactNode }) {
  return (
    <a href={href} target="_blank" rel="noreferrer">
      {children ?? href}
    </a>
  );
}

function Controller() {
  return (
    <address className="not-italic">
      Dmitrii Balakin
      <br />
      Serbia
      <br />
      <Email />
    </address>
  );
}

export default function Privacy() {
  return (
    <HomeLayout {...homeOptions()}>
      <div className="mx-auto w-full max-w-3xl px-6 py-16 md:py-24">
        <article className="prose max-w-none">
          <h1>Privacy Policy</h1>
          <p className="text-fd-muted-foreground">
            Last updated: <time dateTime={lastUpdatedIso}>{lastUpdated}</time>
          </p>

          <h2>1. Introduction</h2>
          <p>
            {appName} is an open-source tool that runs AI coding agents in isolated virtual machine
            environments, published under the{' '}
            <External href={licenseUrl}>Apache License 2.0</External>. This policy explains how
            personal data is processed when you visit the {appName} website at{' '}
            <Link to="/">solitary.balakin.io</Link>, read its documentation, and when you contact
            the project voluntarily. It covers the website's hosting and its analytics, and
            correspondence you choose to send.
          </p>

          <h2>2. Data controller</h2>
          <p>The individual responsible for the processing described in this policy is:</p>
          <Controller />
          <p>
            Questions and requests about privacy can be sent to <Email />.
          </p>

          <h2>3. Information we collect</h2>
          <h3>Website analytics</h3>
          <p>
            The website uses Umami Cloud to understand how it is used. When you load a page, Umami
            processes information about the visit, including:
          </p>
          <ul>
            <li>the pages viewed and the time of each view;</li>
            <li>the website that referred you, if your browser reports one;</li>
            <li>your browser, operating system, device type, screen size and language;</li>
            <li>your approximate geographic location, such as country and region;</li>
            <li>
              a small number of interactions with the site: following a link to the {appName}{' '}
              repository on GitHub, and opening the installation or getting-started instructions,
              including copying the install command;
            </li>
            <li>visitor and session identifiers that are pseudonymous and rotate periodically.</li>
          </ul>
          <p>
            Page addresses are recorded without their query string or fragment. Umami may process
            your IP address to derive information such as your approximate location and the session
            identifiers above, but it does not store the raw IP address in standard analytics
            records. IP addresses may also be processed separately for infrastructure and security
            purposes. Because some of this information relates to an individual browser, analytics
            data is treated in this policy as pseudonymous personal data, not as anonymous data.
          </p>
          <p>
            When your browser sends a Do Not Track signal, the analytics script sends nothing, and
            it does not run at all if a content blocker prevents it from loading. The website works
            the same either way.
          </p>
          <h3>Website hosting</h3>
          <p>
            The website is hosted on GitHub Pages. To deliver pages and protect its service, GitHub
            processes technical information about each request, including your IP address, the
            address requested, and your browser's user agent.
          </p>
          <h3>Voluntary communications</h3>
          <p>
            If you email the project or take part in its GitHub issues, discussions or pull
            requests, the information you choose to provide — such as your name, email address,
            GitHub username and the content of your message — is processed in order to handle it.
            Contributions made on GitHub are also subject to GitHub's own terms and privacy
            statement, and are generally public.
          </p>

          <h2>4. Application telemetry</h2>
          <p>
            <strong>
              The {appName} application does not collect or transmit usage analytics to the project
              maintainer.
            </strong>
          </p>
          <p>
            The command-line tool asks GitHub, at most once a day, whether a newer release exists.
            That request goes to GitHub, carries no information about how {appName} is used, and can
            be turned off by setting <code>SOLITARY_NO_UPDATE_CHECK</code>. Release files are
            downloaded from GitHub, which shows maintainers aggregate download counts that identify
            no one.
          </p>
          <p>
            This section describes only what the {appName} project collects. It does not cover
            processing by AI providers, container images, package registries or other software and
            services you choose to use with {appName}; their own terms and privacy policies apply.
          </p>

          <h2>5. Purposes and legal bases</h2>
          <p>Personal data described in this policy is processed in order to:</p>
          <ul>
            <li>operate the website and keep it secure;</li>
            <li>understand how much traffic the website receives and where it comes from;</li>
            <li>improve the {appName} project and its documentation;</li>
            <li>respond to correspondence and feedback;</li>
            <li>meet applicable legal obligations.</li>
          </ul>
          <p>
            Where the EU General Data Protection Regulation (GDPR) or a comparable law applies, the
            first four purposes rely on legitimate interests under Article 6(1)(f): running a secure
            website, measuring interest in an open-source project in a way that minimises the data
            involved, and answering people who get in touch. These interests are balanced against
            your rights, which is why analytics is limited as described above. Processing needed to
            comply with a legal obligation relies on Article 6(1)(c). Where applicable law requires
            consent for any of this processing, consent will be obtained before it takes place.
          </p>

          <h2>6. Cookies and tracking technologies</h2>
          <p>
            The analytics in use does not set cookies. Umami's script can read a single setting from
            your browser's local storage, which lets a site owner exclude their own browser from
            analytics; it stores no identifier there. The website does not use behavioural
            advertising trackers or persistent cross-site marketing identifiers.
          </p>
          <p>
            Cookie-free analytics still processes personal data, as described in section 3, and this
            policy applies to it in the same way.
          </p>

          <h2>7. Third-party service providers</h2>
          <h3>GitHub Pages</h3>
          <p>
            Provided by GitHub, Inc. and its applicable affiliates, for website hosting and for the
            delivery and security of that infrastructure. See the{' '}
            <External href="https://docs.github.com/en/site-policy/privacy-policies/github-general-privacy-statement">
              GitHub General Privacy Statement
            </External>
            .
          </p>
          <h3>Umami Cloud</h3>
          <p>
            Operated by Umami Software, Inc., for website analytics, with analytics data stored
            primarily in the European Union. See the{' '}
            <External href="https://umami.is/privacy">Umami privacy policy</External> and{' '}
            <External href="https://umami.is/dpa">data processing agreement</External>.
          </p>
          <p>
            Links on this website, including links to the{' '}
            <External href={repoUrl}>{appName} repository</External> on GitHub, lead to services
            that are operated independently and have their own privacy practices.
          </p>

          <h2>8. International data transfers</h2>
          <p>
            Analytics data is stored primarily in the European Union. GitHub and Umami may carry out
            some operational, infrastructure, security or support processing in other countries,
            including the United States. Where personal data is transferred to a country that does
            not provide an adequate level of protection under applicable law, these providers state
            that they rely on recognised transfer mechanisms, such as standard contractual clauses
            or an applicable adequacy decision. Details are available in each provider's
            documentation linked above.
          </p>

          <h2>9. Data retention</h2>
          <ul>
            <li>
              <strong>Website analytics:</strong> retained for six months under the Umami Cloud
              Hobby plan, after which it is deleted.
            </li>
            <li>
              <strong>Correspondence and feedback:</strong> kept only as long as reasonably
              necessary to handle the correspondence, maintain the project, resolve disputes or meet
              legal obligations, and reviewed periodically.
            </li>
            <li>
              <strong>Hosting and security logs:</strong> managed by GitHub according to its
              applicable practices.
            </li>
          </ul>

          <h2>10. Data security</h2>
          <p>
            The website is served only over HTTPS. Administrative access to the website, its
            repository and its analytics is restricted, the data collected is kept to what the
            purposes above require, and hosting and analytics are provided by established services.
            No method of transmission or storage is completely secure, and absolute security cannot
            be guaranteed.
          </p>

          <h2>11. Your data protection rights</h2>
          <p>
            Depending on the law that applies to you, you may have the right to access, rectify or
            erase your personal data, to restrict or object to its processing, to data portability,
            to withdraw consent where processing is based on it, and to lodge a complaint with a
            data protection supervisory authority.
          </p>
          <p>
            {appName} does not keep directly identifying website analytics records, so it may not be
            possible to find the analytics records relating to you. In line with Article 11 of the
            GDPR, additional information will not be collected solely to identify you. If you
            provide information that makes identification possible, your request will be assessed
            and honoured as applicable law requires. This limitation does not apply to identifiable
            correspondence, such as emails you have sent.
          </p>
          <p>
            To exercise your rights, contact <Email />.
          </p>

          <h2>12. Automated decision-making</h2>
          <p>
            Website analytics is not used to make automated decisions that produce legal or
            similarly significant effects, or to build profiles for behavioural advertising.
          </p>

          <h2>13. Children's privacy</h2>
          <p>
            {appName} is intended primarily for developers and other technical users. It does not
            knowingly collect children's personal information for marketing or profiling.
          </p>

          <h2>14. Changes to this policy</h2>
          <p>
            This policy may be updated from time to time. When it changes substantively, the "Last
            updated" date at the top of this page will be revised.
          </p>

          <h2>15. Contact</h2>
          <Controller />
        </article>
      </div>
      <Footer />
    </HomeLayout>
  );
}
