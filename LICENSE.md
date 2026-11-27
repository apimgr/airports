MIT License

Copyright (c) 2024-2026 apimgr

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

---

## Third-Party Licenses

### Airport Data

#### Airport records

Source: [OurAirports](https://ourairports.com/data/), distributed through the [ourairports-data repository](https://github.com/davidmegginson/ourairports-data)
Version: commit `a537e082ac8c8fe6ab509df1f128cc6704fe73c1`
Retrieved: 2026-11-26
License: Public Domain, as stated by OurAirports

The embedded snapshot includes only OurAirports `small_airport`, `medium_airport`, and `large_airport` records. Heliports, seaplane bases, balloonports, and closed facilities are excluded. `airports.csv` supplies airport fields and `regions.csv` supplies region names. Fields are mapped to the project schema.

#### Timezone values

Source: [mwgg/Airports](https://github.com/mwgg/Airports)
Version: commit `a89e4e0dda9d9878337e2da190ebe1ef1d6af705`
Retrieved: 2026-11-26
License: MIT

Timezone identifiers are joined by OurAirports `ident`. The field is omitted when no match exists. The upstream README says timezone data was initially sourced from [TimeZoneDB](https://www.timezonedb.com/) and updated using [TimeAPI](https://timeapi.io/). TimeZoneDB states its database is licensed under CC BY 3.0; TimeAPI is recorded as upstream lineage, and no separate TimeAPI license is asserted here. The timezone values are redistributed from the pinned mwgg/Airports dataset under its MIT package license, with TimeZoneDB attribution retained.

Each embedded record has a `source` key naming `OurAirports` and, when a timezone value is joined, `mwgg/Airports (timezone)`. These internal keys are not returned by API responses. Source file hashes and transformation details are recorded in `src/data/airports.provenance.json`.

#### mwgg/Airports MIT License

The complete MIT license text from the pinned source version follows.

The MIT License (MIT)

Copyright (c) 2014 mwgg

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the \"Software\"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

#### TimeZoneDB CC BY 3.0 License

TimeZoneDB data attribution: [TimeZoneDB](https://www.timezonedb.com/), licensed under the [Creative Commons Attribution 3.0 Unported License](https://creativecommons.org/licenses/by/3.0/legalcode).

The full license text follows.

Creative Commons Legal Code

Attribution 3.0 Unported

    CREATIVE COMMONS CORPORATION IS NOT A LAW FIRM AND DOES NOT PROVIDE
    LEGAL SERVICES. DISTRIBUTION OF THIS LICENSE DOES NOT CREATE AN
    ATTORNEY-CLIENT RELATIONSHIP. CREATIVE COMMONS PROVIDES THIS
    INFORMATION ON AN "AS-IS" BASIS. CREATIVE COMMONS MAKES NO WARRANTIES
    REGARDING THE INFORMATION PROVIDED, AND DISCLAIMS LIABILITY FOR
    DAMAGES RESULTING FROM ITS USE.

License

THE WORK (AS DEFINED BELOW) IS PROVIDED UNDER THE TERMS OF THIS CREATIVE
COMMONS PUBLIC LICENSE ("CCPL" OR "LICENSE"). THE WORK IS PROTECTED BY
COPYRIGHT AND/OR OTHER APPLICABLE LAW. ANY USE OF THE WORK OTHER THAN AS
AUTHORIZED UNDER THIS LICENSE OR COPYRIGHT LAW IS PROHIBITED.

BY EXERCISING ANY RIGHTS TO THE WORK PROVIDED HERE, YOU ACCEPT AND AGREE
TO BE BOUND BY THE TERMS OF THIS LICENSE. TO THE EXTENT THIS LICENSE MAY
BE CONSIDERED TO BE A CONTRACT, THE LICENSOR GRANTS YOU THE RIGHTS
CONTAINED HERE IN CONSIDERATION OF YOUR ACCEPTANCE OF SUCH TERMS AND
CONDITIONS.

1. Definitions

 a. "Adaptation" means a work based upon the Work, or upon the Work and
    other pre-existing works, such as a translation, adaptation,
    derivative work, arrangement of music or other alterations of a
    literary or artistic work, or phonogram or performance and includes
    cinematographic adaptations or any other form in which the Work may be
    recast, transformed, or adapted including in any form recognizably
    derived from the original, except that a work that constitutes a
    Collection will not be considered an Adaptation for the purpose of
    this License. For the avoidance of doubt, where the Work is a musical
    work, performance or phonogram, the synchronization of the Work in
    timed-relation with a moving image ("synching") will be considered an
    Adaptation for the purpose of this License.
 b. "Collection" means a collection of literary or artistic works, such as
    encyclopedias and anthologies, or performances, phonograms or
    broadcasts, or other works or subject matter other than works listed
    in Section 1(f) below, which, by reason of the selection and
    arrangement of their contents, constitute intellectual creations, in
    which the Work is included in its entirety in unmodified form along
    with one or more other contributions, each constituting separate and
    independent works in themselves, which together are assembled into a
    collective whole. A work that constitutes a Collection will not be
    considered an Adaptation (as defined above) for the purposes of this
    License.
 c. "Distribute" means to make available to the public the original and
    copies of the Work or Adaptation, as appropriate, through sale or
    other transfer of ownership.
 d. "Licensor" means the individual, individuals, entity or entities that
    offer(s) the Work under the terms of this License.
 e. "Original Author" means, in the case of a literary or artistic work,
    the individual, individuals, entity or entities who created the Work
    or if no individual or entity can be identified, the publisher; and in
    addition (i) in the case of a performance the actors, singers,
    musicians, dancers, and other persons who act, sing, deliver, declaim,
    play in, interpret or otherwise perform literary or artistic works or
    expressions of folklore; (ii) in the case of a phonogram the producer
    being the person or legal entity who first fixes the sounds of a
    performance or other sounds; and, (iii) in the case of broadcasts, the
    organization that transmits the broadcast.
 f. "Work" means the literary and/or artistic work offered under the terms
    of this License including without limitation any production in the
    literary, scientific and artistic domain, whatever may be the mode or
    form of its expression including digital form, such as a book,
    pamphlet and other writing; a lecture, address, sermon or other work
    of the same nature; a dramatic or dramatico-musical work; a
    choreographic work or entertainment in dumb show; a musical
    composition with or without words; a cinematographic work to which are
    assimilated works expressed by a process analogous to cinematography;
    a work of drawing, painting, architecture, sculpture, engraving or
    lithography; a photographic work to which are assimilated works
    expressed by a process analogous to photography; a work of applied
    art; an illustration, map, plan, sketch or three-dimensional work
    relative to geography, topography, architecture or science; a
    performance; a broadcast; a phonogram; a compilation of data to the
    extent it is protected as a copyrightable work; or a work performed by
    a variety or circus performer to the extent it is not otherwise
    considered a literary or artistic work.
 g. "You" means an individual or entity exercising rights under this
    License who has not previously violated the terms of this License with
    respect to the Work, or who has received express permission from the
    Licensor to exercise rights under this License despite a previous
    violation.
 h. "Publicly Perform" means to perform public recitations of the Work and
    to communicate to the public those public recitations, by any means or
    process, including by wire or wireless means or public digital
    performances; to make available to the public Works in such a way that
    members of the public may access these Works from a place and at a
    place individually chosen by them; to perform the Work to the public
    by any means or process and the communication to the public of the
    performances of the Work, including by public digital performance; to
    broadcast and rebroadcast the Work by any means including signs,
    sounds or images.
 i. "Reproduce" means to make copies of the Work by any means including
    without limitation by sound or visual recordings and the right of
    fixation and reproducing fixations of the Work, including storage of a
    protected performance or phonogram in digital form or other electronic
    medium.

2. Fair Dealing Rights. Nothing in this License is intended to reduce,
limit, or restrict any uses free from copyright or rights arising from
limitations or exceptions that are provided for in connection with the
copyright protection under copyright law or other applicable laws.

3. License Grant. Subject to the terms and conditions of this License,
Licensor hereby grants You a worldwide, royalty-free, non-exclusive,
perpetual (for the duration of the applicable copyright) license to
exercise the rights in the Work as stated below:

 a. to Reproduce the Work, to incorporate the Work into one or more
    Collections, and to Reproduce the Work as incorporated in the
    Collections;
 b. to create and Reproduce Adaptations provided that any such Adaptation,
    including any translation in any medium, takes reasonable steps to
    clearly label, demarcate or otherwise identify that changes were made
    to the original Work. For example, a translation could be marked "The
    original work was translated from English to Spanish," or a
    modification could indicate "The original work has been modified.";
 c. to Distribute and Publicly Perform the Work including as incorporated
    in Collections; and,
 d. to Distribute and Publicly Perform Adaptations.
 e. For the avoidance of doubt:

     i. Non-waivable Compulsory License Schemes. In those jurisdictions in
        which the right to collect royalties through any statutory or
        compulsory licensing scheme cannot be waived, the Licensor
        reserves the exclusive right to collect such royalties for any
        exercise by You of the rights granted under this License;
    ii. Waivable Compulsory License Schemes. In those jurisdictions in
        which the right to collect royalties through any statutory or
        compulsory licensing scheme can be waived, the Licensor waives the
        exclusive right to collect such royalties for any exercise by You
        of the rights granted under this License; and,
   iii. Voluntary License Schemes. The Licensor waives the right to
        collect royalties, whether individually or, in the event that the
        Licensor is a member of a collecting society that administers
        voluntary licensing schemes, via that society, from any exercise
        by You of the rights granted under this License.

The above rights may be exercised in all media and formats whether now
known or hereafter devised. The above rights include the right to make
such modifications as are technically necessary to exercise the rights in
other media and formats. Subject to Section 8(f), all rights not expressly
granted by Licensor are hereby reserved.

4. Restrictions. The license granted in Section 3 above is expressly made
subject to and limited by the following restrictions:

 a. You may Distribute or Publicly Perform the Work only under the terms
    of this License. You must include a copy of, or the Uniform Resource
    Identifier (URI) for, this License with every copy of the Work You
    Distribute or Publicly Perform. You may not offer or impose any terms
    on the Work that restrict the terms of this License or the ability of
    the recipient of the Work to exercise the rights granted to that
    recipient under the terms of the License. You may not sublicense the
    Work. You must keep intact all notices that refer to this License and
    to the disclaimer of warranties with every copy of the Work You
    Distribute or Publicly Perform. When You Distribute or Publicly
    Perform the Work, You may not impose any effective technological
    measures on the Work that restrict the ability of a recipient of the
    Work from You to exercise the rights granted to that recipient under
    the terms of the License. This Section 4(a) applies to the Work as
    incorporated in a Collection, but this does not require the Collection
    apart from the Work itself to be made subject to the terms of this
    License. If You create a Collection, upon notice from any Licensor You
    must, to the extent practicable, remove from the Collection any credit
    as required by Section 4(b), as requested. If You create an
    Adaptation, upon notice from any Licensor You must, to the extent
    practicable, remove from the Adaptation any credit as required by
    Section 4(b), as requested.
 b. If You Distribute, or Publicly Perform the Work or any Adaptations or
    Collections, You must, unless a request has been made pursuant to
    Section 4(a), keep intact all copyright notices for the Work and
    provide, reasonable to the medium or means You are utilizing: (i) the
    name of the Original Author (or pseudonym, if applicable) if supplied,
    and/or if the Original Author and/or Licensor designate another party
    or parties (e.g., a sponsor institute, publishing entity, journal) for
    attribution ("Attribution Parties") in Licensor's copyright notice,
    terms of service or by other reasonable means, the name of such party
    or parties; (ii) the title of the Work if supplied; (iii) to the
    extent reasonably practicable, the URI, if any, that Licensor
    specifies to be associated with the Work, unless such URI does not
    refer to the copyright notice or licensing information for the Work;
    and (iv) , consistent with Section 3(b), in the case of an Adaptation,
    a credit identifying the use of the Work in the Adaptation (e.g.,
    "French translation of the Work by Original Author," or "Screenplay
    based on original Work by Original Author"). The credit required by
    this Section 4 (b) may be implemented in any reasonable manner;
    provided, however, that in the case of a Adaptation or Collection, at
    a minimum such credit will appear, if a credit for all contributing
    authors of the Adaptation or Collection appears, then as part of these
    credits and in a manner at least as prominent as the credits for the
    other contributing authors. For the avoidance of doubt, You may only
    use the credit required by this Section for the purpose of attribution
    in the manner set out above and, by exercising Your rights under this
    License, You may not implicitly or explicitly assert or imply any
    connection with, sponsorship or endorsement by the Original Author,
    Licensor and/or Attribution Parties, as appropriate, of You or Your
    use of the Work, without the separate, express prior written
    permission of the Original Author, Licensor and/or Attribution
    Parties.
 c. Except as otherwise agreed in writing by the Licensor or as may be
    otherwise permitted by applicable law, if You Reproduce, Distribute or
    Publicly Perform the Work either by itself or as part of any
    Adaptations or Collections, You must not distort, mutilate, modify or
    take other derogatory action in relation to the Work which would be
    prejudicial to the Original Author's honor or reputation. Licensor
    agrees that in those jurisdictions (e.g. Japan), in which any exercise
    of the right granted in Section 3(b) of this License (the right to
    make Adaptations) would be deemed to be a distortion, mutilation,
    modification or other derogatory action prejudicial to the Original
    Author's honor and reputation, the Licensor will waive or not assert,
    as appropriate, this Section, to the fullest extent permitted by the
    applicable national law, to enable You to reasonably exercise Your
    right under Section 3(b) of this License (right to make Adaptations)
    but not otherwise.

5. Representations, Warranties and Disclaimer

UNLESS OTHERWISE MUTUALLY AGREED TO BY THE PARTIES IN WRITING, LICENSOR
OFFERS THE WORK AS-IS AND MAKES NO REPRESENTATIONS OR WARRANTIES OF ANY
KIND CONCERNING THE WORK, EXPRESS, IMPLIED, STATUTORY OR OTHERWISE,
INCLUDING, WITHOUT LIMITATION, WARRANTIES OF TITLE, MERCHANTIBILITY,
FITNESS FOR A PARTICULAR PURPOSE, NONINFRINGEMENT, OR THE ABSENCE OF
LATENT OR OTHER DEFECTS, ACCURACY, OR THE PRESENCE OF ABSENCE OF ERRORS,
WHETHER OR NOT DISCOVERABLE. SOME JURISDICTIONS DO NOT ALLOW THE EXCLUSION
OF IMPLIED WARRANTIES, SO SUCH EXCLUSION MAY NOT APPLY TO YOU.

6. Limitation on Liability. EXCEPT TO THE EXTENT REQUIRED BY APPLICABLE
LAW, IN NO EVENT WILL LICENSOR BE LIABLE TO YOU ON ANY LEGAL THEORY FOR
ANY SPECIAL, INCIDENTAL, CONSEQUENTIAL, PUNITIVE OR EXEMPLARY DAMAGES
ARISING OUT OF THIS LICENSE OR THE USE OF THE WORK, EVEN IF LICENSOR HAS
BEEN ADVISED OF THE POSSIBILITY OF SUCH DAMAGES.

7. Termination

 a. This License and the rights granted hereunder will terminate
    automatically upon any breach by You of the terms of this License.
    Individuals or entities who have received Adaptations or Collections
    from You under this License, however, will not have their licenses
    terminated provided such individuals or entities remain in full
    compliance with those licenses. Sections 1, 2, 5, 6, 7, and 8 will
    survive any termination of this License.
 b. Subject to the above terms and conditions, the license granted here is
    perpetual (for the duration of the applicable copyright in the Work).
    Notwithstanding the above, Licensor reserves the right to release the
    Work under different license terms or to stop distributing the Work at
    any time; provided, however that any such election will not serve to
    withdraw this License (or any other license that has been, or is
    required to be, granted under the terms of this License), and this
    License will continue in full force and effect unless terminated as
    stated above.

8. Miscellaneous

 a. Each time You Distribute or Publicly Perform the Work or a Collection,
    the Licensor offers to the recipient a license to the Work on the same
    terms and conditions as the license granted to You under this License.
 b. Each time You Distribute or Publicly Perform an Adaptation, Licensor
    offers to the recipient a license to the original Work on the same
    terms and conditions as the license granted to You under this License.
 c. If any provision of this License is invalid or unenforceable under
    applicable law, it shall not affect the validity or enforceability of
    the remainder of the terms of this License, and without further action
    by the parties to this agreement, such provision shall be reformed to
    the minimum extent necessary to make such provision valid and
    enforceable.
 d. No term or provision of this License shall be deemed waived and no
    breach consented to unless such waiver or consent shall be in writing
    and signed by the party to be charged with such waiver or consent.
 e. This License constitutes the entire agreement between the parties with
    respect to the Work licensed here. There are no understandings,
    agreements or representations with respect to the Work not specified
    here. Licensor shall not be bound by any additional provisions that
    may appear in any communication from You. This License may not be
    modified without the mutual written agreement of the Licensor and You.
 f. The rights granted under, and the subject matter referenced, in this
    License were drafted utilizing the terminology of the Berne Convention
    for the Protection of Literary and Artistic Works (as amended on
    September 28, 1979), the Rome Convention of 1961, the WIPO Copyright
    Treaty of 1996, the WIPO Performances and Phonograms Treaty of 1996
    and the Universal Copyright Convention (as revised on July 24, 1971).
    These rights and subject matter take effect in the relevant
    jurisdiction in which the License terms are sought to be enforced
    according to the corresponding provisions of the implementation of
    those treaty provisions in the applicable national law. If the
    standard suite of rights granted under applicable copyright law
    includes additional rights not granted under this License, such
    additional rights are deemed to be included in the License; this
    License is not intended to restrict the license of any rights under
    applicable law.


Creative Commons Notice

    Creative Commons is not a party to this License, and makes no warranty
    whatsoever in connection with the Work. Creative Commons will not be
    liable to You or any party on any legal theory for any damages
    whatsoever, including without limitation any general, special,
    incidental or consequential damages arising in connection to this
    license. Notwithstanding the foregoing two (2) sentences, if Creative
    Commons has expressly identified itself as the Licensor hereunder, it
    shall have all rights and obligations of Licensor.

    Except for the limited purpose of indicating to the public that the
    Work is licensed under the CCPL, Creative Commons does not authorize
    the use by either party of the trademark "Creative Commons" or any
    related trademark or logo of Creative Commons without the prior
    written consent of Creative Commons. Any permitted use will be in
    compliance with Creative Commons' then-current trademark usage
    guidelines, as may be published on its website or otherwise made
    available upon request from time to time. For the avoidance of doubt,
    this trademark restriction does not form part of this License.

    Creative Commons may be contacted at https://creativecommons.org/.

### GeoIP Databases

Source: [sapics/ip-location-db](https://github.com/sapics/ip-location-db)
License: PDDL (ASN and country data); CC BY 4.0 (DB-IP city data)

| Database | License | Underlying Source(s) |
|----------|---------|-----------------------|
| ASN | PDDL | iptoasn.com (RouteViews / RIR data) |
| Country | PDDL | iptoasn.com (RIR data) |
| City | CC BY 4.0 | DB-IP |

The DB-IP attribution is required because the City database is used. The ASN
and country datasets are PDDL and require no attribution. MaxMind GeoLite2 is
not used by this project:

[IP Geolocation by DB-IP](https://db-ip.com/)

### Go Dependencies

This inventory covers dependencies reported by `go-licenses csv ./src/...`, matching the package scope used by the license workflow. Linked upstream license files retain the complete copyright notices. `modernc.org/mathutil` v1.7.1 is the sole scanner exception: its module archive omits a license file, while the upstream project identifies BSD-3-Clause.

| Library/package | Version | License | Copyright/attribution | Upstream license |
|---------|---------|---------|---------|---------|
| `cloud.google.com/go/auth` | v0.20.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/googleapis/google-cloud-go/blob/auth/v0.20.0/auth/LICENSE) |
| `cloud.google.com/go/auth/oauth2adapt` | v0.2.8 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/googleapis/google-cloud-go/blob/auth/oauth2adapt/v0.2.8/auth/oauth2adapt/LICENSE) |
| `cloud.google.com/go/compute/metadata` | v0.9.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/googleapis/google-cloud-go/blob/compute/metadata/v0.9.0/compute/metadata/LICENSE) |
| `github.com/AdamSLevy/jsonrpc2/v14` | v14.1.0 | MIT | Copyright 2018 Adam S Levy | [notice](https://github.com/AdamSLevy/jsonrpc2/blob/v14.1.0/LICENSE) |
| `github.com/Azure/azure-sdk-for-go` | v68.0.0+incompatible | MIT | Copyright (c) Microsoft Corporation. | [notice](https://github.com/Azure/azure-sdk-for-go/blob/v68.0.0/LICENSE.txt) |
| `github.com/Azure/azure-sdk-for-go/sdk/azcore` | v1.21.1 | MIT | Copyright (c) Microsoft Corporation. | [notice](https://github.com/Azure/azure-sdk-for-go/blob/sdk/azcore/v1.21.1/sdk/azcore/LICENSE.txt) |
| `github.com/Azure/azure-sdk-for-go/sdk/azidentity` | v1.13.1 | MIT | Copyright (c) Microsoft Corporation. | [notice](https://github.com/Azure/azure-sdk-for-go/blob/sdk/azidentity/v1.13.1/sdk/azidentity/LICENSE.txt) |
| `github.com/Azure/azure-sdk-for-go/sdk/internal` | v1.12.0 | MIT | Copyright (c) Microsoft Corporation. | [notice](https://github.com/Azure/azure-sdk-for-go/blob/sdk/internal/v1.12.0/sdk/internal/LICENSE.txt) |
| `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns` | v1.2.0 | MIT | Copyright (c) Microsoft Corporation. All rights reserved. | [notice](https://github.com/Azure/azure-sdk-for-go/blob/sdk/resourcemanager/dns/armdns/v1.2.0/sdk/resourcemanager/dns/armdns/LICENSE.txt) |
| `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/privatedns/armprivatedns` | v1.3.0 | MIT | Copyright (c) Microsoft Corporation. All rights reserved. | [notice](https://github.com/Azure/azure-sdk-for-go/blob/sdk/resourcemanager/privatedns/armprivatedns/v1.3.0/sdk/resourcemanager/privatedns/armprivatedns/LICENSE.txt) |
| `github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph` | v0.9.0 | MIT | Copyright (c) Microsoft Corporation. All rights reserved. | [notice](https://github.com/Azure/azure-sdk-for-go/blob/sdk/resourcemanager/resourcegraph/armresourcegraph/v0.9.0/sdk/resourcemanager/resourcegraph/armresourcegraph/LICENSE.txt) |
| `github.com/Azure/go-autorest/autorest` | v0.11.30 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/Azure/go-autorest/blob/autorest/v0.11.30/autorest/LICENSE) |
| `github.com/Azure/go-autorest/autorest/adal` | v0.9.22 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/Azure/go-autorest/blob/autorest/adal/v0.9.22/autorest/adal/LICENSE) |
| `github.com/Azure/go-autorest/autorest/azure/auth` | v0.5.13 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/Azure/go-autorest/blob/autorest/azure/auth/v0.5.13/autorest/azure/auth/LICENSE) |
| `github.com/Azure/go-autorest/autorest/azure/cli` | v0.4.6 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/Azure/go-autorest/blob/autorest/azure/cli/v0.4.6/autorest/azure/cli/LICENSE) |
| `github.com/Azure/go-autorest/autorest/date` | v0.3.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/Azure/go-autorest/blob/autorest/date/v0.3.0/autorest/date/LICENSE) |
| `github.com/Azure/go-autorest/autorest/to` | v0.4.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/Azure/go-autorest/blob/autorest/to/v0.4.1/autorest/to/LICENSE) |
| `github.com/Azure/go-autorest/logger` | v0.2.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/Azure/go-autorest/blob/logger/v0.2.1/logger/LICENSE) |
| `github.com/Azure/go-autorest/tracing` | v0.6.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/Azure/go-autorest/blob/tracing/v0.6.0/tracing/LICENSE) |
| `github.com/AzureAD/microsoft-authentication-library-for-go/apps` | v1.6.0 | MIT | Copyright (c) Microsoft Corporation. | [notice](https://github.com/AzureAD/microsoft-authentication-library-for-go/blob/v1.6.0/LICENSE) |
| `github.com/akamai/AkamaiOPEN-edgegrid-golang/v13/pkg` | v13.1.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/akamai/AkamaiOPEN-edgegrid-golang/blob/v13.1.0/LICENSE) |
| `github.com/alibabacloud-go/alibabacloud-gateway-spi/client` | v0.0.5 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/alibabacloud-go/alibabacloud-gateway-spi/blob/v0.0.5/LICENSE) |
| `github.com/alibabacloud-go/darabonba-openapi/v2` | v2.1.16 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/alibabacloud-go/darabonba-openapi/blob/v2.1.16/LICENSE) |
| `github.com/alibabacloud-go/debug/debug` | v1.0.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/alibabacloud-go/debug/blob/v1.0.1/LICENSE) |
| `github.com/alibabacloud-go/tea` | v1.4.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/alibabacloud-go/tea/blob/v1.4.0/LICENSE) |
| `github.com/alibabacloud-go/tea-utils/v2/service` | v2.0.9 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/alibabacloud-go/tea-utils/blob/v2.0.9/LICENSE) |
| `github.com/aliyun/credentials-go/credentials` | v1.4.7 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aliyun/credentials-go/blob/v1.4.7/LICENSE) |
| `github.com/aws/aws-sdk-go-v2` | v1.41.6 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/v1.41.6/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/config` | v1.32.16 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/config/v1.32.16/config/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/credentials` | v1.19.15 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/credentials/v1.19.15/credentials/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/feature/ec2/imds` | v1.18.22 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/feature/ec2/imds/v1.18.22/feature/ec2/imds/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/internal/configsources` | v1.4.22 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/internal/configsources/v1.4.22/internal/configsources/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/internal/endpoints/v2` | v2.7.22 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/internal/endpoints/v2.7.22/internal/endpoints/v2/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/internal/sync/singleflight` | v1.41.6 | BSD-3-Clause | Copyright (c) 2009 The Go Authors. All rights reserved. | [notice](https://github.com/aws/aws-sdk-go-v2/blob/v1.41.6/internal/sync/singleflight/LICENSE) |
| `github.com/aws/aws-sdk-go-v2/internal/v4a` | v1.4.23 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/internal/v4a/v1.4.23/internal/v4a/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding` | v1.13.8 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/service/internal/accept-encoding/v1.13.8/service/internal/accept-encoding/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/service/internal/presigned-url` | v1.13.22 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/service/internal/presigned-url/v1.13.22/service/internal/presigned-url/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/service/lightsail` | v1.53.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/service/lightsail/v1.53.1/service/lightsail/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/service/route53` | v1.62.6 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/service/route53/v1.62.6/service/route53/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/service/signin` | v1.0.10 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/service/signin/v1.0.10/service/signin/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/service/sso` | v1.30.16 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/service/sso/v1.30.16/service/sso/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/service/ssooidc` | v1.35.20 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/service/ssooidc/v1.35.20/service/ssooidc/LICENSE.txt) |
| `github.com/aws/aws-sdk-go-v2/service/sts` | v1.42.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/aws-sdk-go-v2/blob/service/sts/v1.42.0/service/sts/LICENSE.txt) |
| `github.com/aws/smithy-go` | v1.25.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/aws/smithy-go/blob/v1.25.0/LICENSE) |
| `github.com/aws/smithy-go/internal/sync/singleflight` | v1.25.0 | BSD-3-Clause | Copyright (c) 2009 The Go Authors. All rights reserved. | [notice](https://github.com/aws/smithy-go/blob/v1.25.0/internal/sync/singleflight/LICENSE) |
| `github.com/aymerick/douceur` | v0.2.0 | MIT | Copyright (c) 2015 Aymerick JEHANNE | [notice](https://github.com/aymerick/douceur/blob/v0.2.0/LICENSE) |
| `github.com/aziontech/azionapi-go-sdk/idns` | v0.144.0 | MIT | Copyright (c) 2022 Azion Technologies | [notice](https://github.com/aziontech/azionapi-go-sdk/blob/v0.144.0/LICENSE.md) |
| `github.com/baidubce/bce-sdk-go` | v0.9.265 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/baidubce/bce-sdk-go/blob/v0.9.265/LICENSE) |
| `github.com/benbjohnson/clock` | v1.3.5 | MIT | Copyright (c) 2014 Ben Johnson | [notice](https://github.com/benbjohnson/clock/blob/v1.3.5/LICENSE) |
| `github.com/beorn7/perks/quantile` | v1.0.1 | MIT | Copyright (C) 2013 Blake Mizerany | [notice](https://github.com/beorn7/perks/blob/v1.0.1/LICENSE) |
| `github.com/bodgit/tsig` | v1.2.2 | BSD-3-Clause | Copyright (c) 2018, Matt Dainty | [notice](https://github.com/bodgit/tsig/blob/v1.2.2/LICENSE) |
| `github.com/boombuler/barcode` | v1.0.1 | MIT | Copyright (c) 2014 Florian Sundermann | [notice](https://github.com/boombuler/barcode/blob/v1.0.1/LICENSE) |
| `github.com/cenkalti/backoff/v4` | v4.3.0 | MIT | Copyright (c) 2014 Cenk Altı | [notice](https://github.com/cenkalti/backoff/blob/v4.3.0/LICENSE) |
| `github.com/cenkalti/backoff/v5` | v5.0.3 | MIT | Copyright (c) 2014 Cenk Altı | [notice](https://github.com/cenkalti/backoff/blob/v5.0.3/LICENSE) |
| `github.com/cespare/xxhash/v2` | v2.3.0 | MIT | Copyright (c) 2016 Caleb Spare | [notice](https://github.com/cespare/xxhash/blob/v2.3.0/LICENSE.txt) |
| `github.com/cloudflare/circl` | v1.6.3 | BSD-3-Clause | Copyright (c) 2019 Cloudflare. All rights reserved. | [notice](https://github.com/cloudflare/circl/blob/v1.6.3/LICENSE) |
| `github.com/cloudflare/circl/ecc/p384` | v1.6.3 | BSD-3-Clause | Copyright (c) 2018, Brendan McMillion. All rights reserved. | [notice](https://github.com/cloudflare/circl/blob/v1.6.3/ecc/p384/LICENSE) |
| `github.com/clbanning/mxj/v2` | v2.7.0 | MIT | Copyright (c) 2012-2021 Charles Banning <clbanning@gmail.com>.  All rights reserved. | [notice](https://github.com/clbanning/mxj/blob/v2.7.0/LICENSE) |
| `github.com/cretz/bine` | v0.2.0 | MIT | Copyright (c) 2018 Chad Retz | [notice](https://github.com/cretz/bine/blob/v0.2.0/LICENSE) |
| `github.com/dimchansky/utfbom` | v1.1.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/dimchansky/utfbom/blob/v1.1.1/LICENSE) |
| `github.com/dnsimple/dnsimple-go/v4/dnsimple` | v4.0.0 | MIT | Copyright (c) 2014-2024 DNSimple Corporation | [notice](https://github.com/dnsimple/dnsimple-go/blob/v4.0.0/LICENSE.txt) |
| `github.com/dustin/go-humanize` | v1.0.1 | MIT | Copyright (c) 2005-2008  Dustin Sallings <dustin@spy.net> | [notice](https://github.com/dustin/go-humanize/blob/v1.0.1/LICENSE) |
| `github.com/exoscale/egoscale/v3` | v3.1.34 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/exoscale/egoscale/blob/v3.1.34/v3/LICENSE) |
| `github.com/fatih/structs` | v1.1.0 | MIT | Copyright (c) 2014 Fatih Arslan | [notice](https://github.com/fatih/structs/blob/v1.1.0/LICENSE) |
| `github.com/felixge/httpsnoop` | v1.0.4 | MIT | Copyright (c) 2016 Felix Geisendörfer (felix@debuggable.com) | [notice](https://github.com/felixge/httpsnoop/blob/v1.0.4/LICENSE.txt) |
| `github.com/fsnotify/fsnotify` | v1.9.0 | BSD-3-Clause | Copyright © 2012 The Go Authors. All rights reserved. | [notice](https://github.com/fsnotify/fsnotify/blob/v1.9.0/LICENSE) |
| `github.com/gabriel-vasile/mimetype` | v1.4.13 | MIT | Copyright (c) 2018 Gabriel Vasile | [notice](https://github.com/gabriel-vasile/mimetype/blob/v1.4.13/LICENSE) |
| `github.com/ghodss/yaml` | v1.0.0 | MIT | Copyright (c) 2014 Sam Ghods | [notice](https://github.com/ghodss/yaml/blob/v1.0.0/LICENSE) |
| `github.com/go-acme/alidns-20150109/v4/client` | v4.7.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/go-acme/alidns-20150109/blob/v4.7.0/LICENSE) |
| `github.com/go-acme/esa-20240910/v2/client` | v2.48.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/go-acme/esa-20240910/blob/v2.48.0/LICENSE) |
| `github.com/go-acme/jdcloud-sdk-go` | v1.64.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/go-acme/jdcloud-sdk-go/blob/v1.64.0/LICENSE) |
| `github.com/go-acme/jdcloud-sdk-go/services/domainservice/models` | v1.64.0 | Apache-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/go-acme/jdcloud-sdk-go/blob/v1.64.0/services/domainservice/models/Notice.go) |
| `github.com/go-acme/lego/v4` | v4.35.2 | MIT | Copyright (c) 2017-2024 Ludovic Fernandez | [notice](https://github.com/go-acme/lego/blob/v4.35.2/LICENSE) |
| `github.com/go-acme/tencentclouddnspod/v20210323` | v1.3.24 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/go-acme/tencentclouddnspod/blob/v1.3.24/LICENSE) |
| `github.com/go-acme/tencentedgdeone/v20220901` | v1.3.38 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/go-acme/tencentedgdeone/blob/v1.3.38/LICENSE) |
| `github.com/go-chi/chi/v5` | v5.3.0 | MIT | Copyright (c) 2015-present Peter Kieltyka (https://github.com/pkieltyka), Google Inc. | [notice](https://github.com/go-chi/chi/blob/v5.3.0/LICENSE) |
| `github.com/go-errors/errors` | v1.0.1 | MIT | Copyright (c) 2015 Conrad Irwin <conrad@bugsnag.com> | [notice](https://github.com/go-errors/errors/blob/v1.0.1/LICENSE.MIT) |
| `github.com/go-jose/go-jose/v4` | v4.1.4 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/go-jose/go-jose/blob/v4.1.4/LICENSE) |
| `github.com/go-jose/go-jose/v4/json` | v4.1.4 | BSD-3-Clause | Copyright (c) 2012 The Go Authors. All rights reserved. | [notice](https://github.com/go-jose/go-jose/blob/v4.1.4/json/LICENSE) |
| `github.com/go-logr/logr` | v1.4.3 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/go-logr/logr/blob/v1.4.3/LICENSE) |
| `github.com/go-logr/stdr` | v1.2.2 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/go-logr/stdr/blob/v1.2.2/LICENSE) |
| `github.com/go-ozzo/ozzo-validation/v4` | v4.3.0 | MIT | Copyright (c) 2016, Qiang Xue | [notice](https://github.com/go-ozzo/ozzo-validation/blob/v4.3.0/LICENSE) |
| `github.com/go-playground/locales` | v0.14.1 | MIT | Copyright (c) 2016 Go Playground | [notice](https://github.com/go-playground/locales/blob/v0.14.1/LICENSE) |
| `github.com/go-playground/universal-translator` | v0.18.1 | MIT | Copyright (c) 2016 Go Playground | [notice](https://github.com/go-playground/universal-translator/blob/v0.18.1/LICENSE) |
| `github.com/go-playground/validator/v10` | v10.23.0 | MIT | Copyright (c) 2015 Dean Karn | [notice](https://github.com/go-playground/validator/blob/v10.23.0/LICENSE) |
| `github.com/go-resty/resty/v2` | v2.17.2 | MIT | Copyright (c) 2015-2024 Jeevanandam M., https://myjeeva.com <jeeva@myjeeva.com> | [notice](https://github.com/go-resty/resty/blob/v2.17.2/LICENSE) |
| `github.com/go-viper/mapstructure/v2` | v2.5.0 | MIT | Copyright (c) 2013 Mitchell Hashimoto | [notice](https://github.com/go-viper/mapstructure/blob/v2.5.0/LICENSE) |
| `github.com/goccy/go-yaml` | v1.19.2 | MIT | Copyright (c) 2019 Masaaki Goshima | [notice](https://github.com/goccy/go-yaml/blob/v1.19.2/LICENSE) |
| `github.com/gofrs/flock` | v0.13.0 | BSD-3-Clause | Copyright (c) 2018-2025, The Gofrs | [notice](https://github.com/gofrs/flock/blob/v0.13.0/LICENSE) |
| `github.com/gofrs/uuid` | v4.4.0+incompatible | MIT | Copyright (C) 2013-2018 by Maxim Bublis <b@codemonkey.ru> | [notice](https://github.com/gofrs/uuid/blob/v4.4.0/LICENSE) |
| `github.com/golang-jwt/jwt/v4` | v4.5.2 | MIT | Copyright (c) 2012 Dave Grijalva | [notice](https://github.com/golang-jwt/jwt/blob/v4.5.2/LICENSE) |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | MIT | Copyright (c) 2012 Dave Grijalva | [notice](https://github.com/golang-jwt/jwt/blob/v5.3.1/LICENSE) |
| `github.com/golang/protobuf/proto` | v1.5.4 | BSD-3-Clause | Copyright 2010 The Go Authors.  All rights reserved. | [notice](https://github.com/golang/protobuf/blob/v1.5.4/LICENSE) |
| `github.com/google/go-querystring/query` | v1.2.0 | BSD-3-Clause | Copyright (c) 2013 Google. All rights reserved. | [notice](https://github.com/google/go-querystring/blob/v1.2.0/LICENSE) |
| `github.com/google/s2a-go` | v0.1.9 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/google/s2a-go/blob/v0.1.9/LICENSE.md) |
| `github.com/google/uuid` | v1.6.0 | BSD-3-Clause | Copyright (c) 2009,2014 Google Inc. All rights reserved. | [notice](https://github.com/google/uuid/blob/v1.6.0/LICENSE) |
| `github.com/googleapis/enterprise-certificate-proxy/client` | v0.3.14 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/googleapis/enterprise-certificate-proxy/blob/v0.3.14/LICENSE) |
| `github.com/googleapis/gax-go/v2` | v2.21.0 | BSD-3-Clause | Copyright 2016, Google Inc. | [notice](https://github.com/googleapis/gax-go/blob/v2.21.0/v2/LICENSE) |
| `github.com/gophercloud/gophercloud` | v1.14.1 | Apache-2.0 | Copyright 2012-2013 Rackspace, Inc. | [notice](https://github.com/gophercloud/gophercloud/blob/v1.14.1/LICENSE) |
| `github.com/gophercloud/utils` | v0.0.0-20231010081019-80377eca5d56 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/gophercloud/utils/blob/80377eca5d56/LICENSE) |
| `github.com/gorilla/css` | v1.0.1 | BSD-3-Clause | Copyright (c) 2023 The Gorilla Authors. All rights reserved. | [notice](https://github.com/gorilla/css/blob/v1.0.1/LICENSE) |
| `github.com/hashicorp/errwrap` | v1.0.0 | MPL-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/hashicorp/errwrap/blob/v1.0.0/LICENSE) |
| `github.com/hashicorp/go-cleanhttp` | v0.5.2 | MPL-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/hashicorp/go-cleanhttp/blob/v0.5.2/LICENSE) |
| `github.com/hashicorp/go-multierror` | v1.1.1 | MPL-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/hashicorp/go-multierror/blob/v1.1.1/LICENSE) |
| `github.com/hashicorp/go-retryablehttp` | v0.7.8 | MPL-2.0 | Copyright (c) 2015 HashiCorp, Inc. | [notice](https://github.com/hashicorp/go-retryablehttp/blob/v0.7.8/LICENSE) |
| `github.com/hashicorp/go-uuid` | v1.0.3 | MPL-2.0 | Copyright © 2015-2022 HashiCorp, Inc. | [notice](https://github.com/hashicorp/go-uuid/blob/v1.0.3/LICENSE) |
| `github.com/hashicorp/hcl` | v1.0.0 | MPL-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/hashicorp/hcl/blob/v1.0.0/LICENSE) |
| `github.com/huaweicloud/huaweicloud-sdk-go-v3` | v0.1.192 | BSD-2-Clause | Copyright (c) Huawei Technologies Co., Ltd. 2020-present. All rights reserved. | [notice](https://github.com/huaweicloud/huaweicloud-sdk-go-v3/blob/v0.1.192/LICENSE) |
| `github.com/iij/doapi` | v0.0.0-20190504054126-0bbf12d6d7df | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/iij/doapi/blob/0bbf12d6d7df/LICENSE.md) |
| `github.com/infobloxopen/infoblox-go-client/v2` | v2.11.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/infobloxopen/infoblox-go-client/blob/v2.11.0/LICENSE) |
| `github.com/jcmturner/aescts/v2` | v2.0.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/jcmturner/aescts/blob/v2.0.0/v2/LICENSE) |
| `github.com/jcmturner/dnsutils/v2` | v2.0.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/jcmturner/dnsutils/blob/v2.0.0/v2/LICENSE) |
| `github.com/jcmturner/gofork` | v1.7.6 | BSD-3-Clause | Copyright (c) 2009 The Go Authors. All rights reserved. | [notice](https://github.com/jcmturner/gofork/blob/v1.7.6/LICENSE) |
| `github.com/jcmturner/goidentity/v6` | v6.0.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/jcmturner/goidentity/blob/v6.0.1/v6/LICENSE) |
| `github.com/jcmturner/gokrb5/v8` | v8.4.4 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/jcmturner/gokrb5/blob/v8.4.4/v8/LICENSE) |
| `github.com/jcmturner/rpc/v2` | v2.0.3 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/jcmturner/rpc/blob/v2.0.3/v2/LICENSE) |
| `github.com/jinzhu/copier` | v0.4.0 | MIT | Copyright (c) 2015 Jinzhu | [notice](https://github.com/jinzhu/copier/blob/v0.4.0/License) |
| `github.com/json-iterator/go` | v1.1.13-0.20220915233716-71ac16282d12 | MIT | Copyright (c) 2016 json-iterator | [notice](https://github.com/json-iterator/go/blob/71ac16282d12/LICENSE) |
| `github.com/k0kubun/go-ansi` | v0.0.0-20180517002512-3bf9e2903213 | MIT | Copyright (c) 2014 Takashi Kokubun | [notice](https://github.com/k0kubun/go-ansi/blob/3bf9e2903213/LICENSE.txt) |
| `github.com/kolo/xmlrpc` | v0.0.0-20220921171641-a4b6fa1dd06b | MIT | Copyright (C) 2012 Dmitry Maksimov | [notice](https://github.com/kolo/xmlrpc/blob/a4b6fa1dd06b/LICENSE) |
| `github.com/kylelemons/godebug` | v1.1.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/kylelemons/godebug/blob/v1.1.0/LICENSE) |
| `github.com/labbsr0x/bindman-dns-webhook/src` | v1.0.2 | MIT | Copyright (c) 2018 Labbs | [notice](https://github.com/labbsr0x/bindman-dns-webhook/blob/v1.0.2/LICENSE) |
| `github.com/labbsr0x/goh/gohclient` | v1.0.1 | MIT | Copyright (c) 2019 Abilio Esteves | [notice](https://github.com/labbsr0x/goh/blob/v1.0.1/LICENSE) |
| `github.com/leodido/go-urn` | v1.4.0 | MIT | Copyright (c) 2018 Leonardo Di Donato | [notice](https://github.com/leodido/go-urn/blob/v1.4.0/LICENSE) |
| `github.com/linode/linodego` | v1.68.0 | MIT | Copyright (c) 2017 Christopher "Chief" Najewicz | [notice](https://github.com/linode/linodego/blob/v1.68.0/LICENSE) |
| `github.com/liquidweb/liquidweb-cli` | v0.7.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/liquidweb/liquidweb-cli/blob/v0.7.0/LICENSE) |
| `github.com/liquidweb/liquidweb-go` | v1.6.4 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/liquidweb/liquidweb-go/blob/v1.6.4/LICENSE) |
| `github.com/magiconair/properties` | v1.8.7 | BSD-2-Clause | Copyright (c) 2013-2020, Frank Schroeder | [notice](https://github.com/magiconair/properties/blob/v1.8.7/LICENSE.md) |
| `github.com/miekg/dns` | v1.1.72 | BSD-3-Clause | Copyright (c) 2009, The Go Authors. Extensions copyright (c) 2011, Miek Gieben. | [notice](https://github.com/miekg/dns/blob/v1.1.72/LICENSE) |
| `github.com/mimuret/golang-iij-dpf/pkg` | v0.9.1 | MIT | Copyright (c) 2021 Manabu Sonoda | [notice](https://github.com/mimuret/golang-iij-dpf/blob/v0.9.1/LICENSE) |
| `github.com/mitchellh/go-homedir` | v1.1.0 | MIT | Copyright (c) 2013 Mitchell Hashimoto | [notice](https://github.com/mitchellh/go-homedir/blob/v1.1.0/LICENSE) |
| `github.com/mitchellh/mapstructure` | v1.5.0 | MIT | Copyright (c) 2013 Mitchell Hashimoto | [notice](https://github.com/mitchellh/mapstructure/blob/v1.5.0/LICENSE) |
| `github.com/microcosm-cc/bluemonday` | v1.0.27 | BSD-3-Clause | Copyright (c) 2014, David Kitchen | [notice](https://github.com/microcosm-cc/bluemonday/blob/v1.0.27/LICENSE.md) |
| `github.com/modern-go/concurrent` | v0.0.0-20180306012644-bacd9c7ef1dd | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/modern-go/concurrent/blob/bacd9c7ef1dd/LICENSE) |
| `github.com/modern-go/reflect2` | v1.0.3-0.20250322232337-35a7c28c31ee | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/modern-go/reflect2/blob/35a7c28c31ee/LICENSE) |
| `github.com/munnerz/goautoneg` | v0.0.0-20191010083416-a7dc8b61c822 | BSD-3-Clause | Copyright (c) 2011, Open Knowledge Foundation Ltd. | [notice](https://github.com/munnerz/goautoneg/blob/a7dc8b61c822/LICENSE) |
| `github.com/namedotcom/go/v4/namecom` | v4.0.2 | MIT | Copyright (c) 2017 Name.com | [notice](https://github.com/namedotcom/go/blob/v4.0.2/LICENSE) |
| `github.com/nrdcg/auroradns` | v1.2.0 | MPL-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/nrdcg/auroradns/blob/v1.2.0/LICENSE) |
| `github.com/nrdcg/bunny-go` | v0.1.0 | MIT | Copyright (c) 2021 simplesurance GmbH | [notice](https://github.com/nrdcg/bunny-go/blob/v0.1.0/LICENSE) |
| `github.com/nrdcg/desec` | v0.11.1 | MPL-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/nrdcg/desec/blob/v0.11.1/LICENSE) |
| `github.com/nrdcg/dnspod-go` | v0.4.0 | MIT | Copyright (c) 2017 decker | [notice](https://github.com/nrdcg/dnspod-go/blob/v0.4.0/LICENSE) |
| `github.com/nrdcg/freemyip` | v0.3.0 | MPL-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/nrdcg/freemyip/blob/v0.3.0/LICENSE) |
| `github.com/nrdcg/goacmedns` | v0.2.0 | MIT | Copyright (c) 2018 Daniel McCarney | [notice](https://github.com/nrdcg/goacmedns/blob/v0.2.0/LICENSE) |
| `github.com/nrdcg/goinwx` | v0.12.0 | MIT | Copyright (c) 2017 Andrew | [notice](https://github.com/nrdcg/goinwx/blob/v0.12.0/LICENSE) |
| `github.com/nrdcg/mailinabox` | v0.3.0 | MPL-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/nrdcg/mailinabox/blob/v0.3.0/LICENSE) |
| `github.com/nrdcg/namesilo` | v0.5.0 | MPL-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/nrdcg/namesilo/blob/v0.5.0/LICENSE) |
| `github.com/nrdcg/nodion` | v0.1.0 | MPL-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/nrdcg/nodion/blob/v0.1.0/LICENSE) |
| `github.com/nrdcg/oci-go-sdk/common/v1065` | v1065.113.0 | Apache-2.0 | Copyright (c) 2016, 2026, Oracle and/or its affiliates.  All rights reserved. | [notice](https://github.com/nrdcg/oci-go-sdk/blob/common/v1065.113.0/common/LICENSE.txt) |
| `github.com/nrdcg/oci-go-sdk/dns/v1065` | v1065.113.0 | Apache-2.0 | Copyright (c) 2016, 2026, Oracle and/or its affiliates.  All rights reserved. | [notice](https://github.com/nrdcg/oci-go-sdk/blob/dns/v1065.113.0/dns/LICENSE.txt) |
| `github.com/nrdcg/porkbun` | v0.4.0 | MPL-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/nrdcg/porkbun/blob/v0.4.0/LICENSE) |
| `github.com/nrdcg/vegadns` | v0.3.0 | Apache-2.0 | Copyright 2018, Cisco Systems, Inc. | [notice](https://github.com/nrdcg/vegadns/blob/v0.3.0/LICENSE) |
| `github.com/nzdjb/go-metaname` | v1.0.0 | MIT | Copyright (c) 2023 nzdjb | [notice](https://github.com/nzdjb/go-metaname/blob/v1.0.0/LICENSE) |
| `github.com/oschwald/maxminddb-golang` | v1.13.0 | ISC | Copyright (c) 2015, Gregory J. Oschwald <oschwald@gmail.com> | [notice](https://github.com/oschwald/maxminddb-golang/blob/v1.13.0/LICENSE) |
| `github.com/ovh/go-ovh/ovh` | v1.9.0 | BSD-3-Clause | Copyright (c) 2015-2025, OVH SAS | [notice](https://github.com/ovh/go-ovh/blob/v1.9.0/LICENSE) |
| `github.com/pelletier/go-toml/v2` | v2.1.0 | MIT | Copyright (c) 2021 - 2023 Thomas Pelletier | [notice](https://github.com/pelletier/go-toml/blob/v2.1.0/LICENSE) |
| `github.com/peterhellberg/link` | v1.2.0 | MIT | Copyright (c) 2015-2022 Peter Hellberg https://c7.se | [notice](https://github.com/peterhellberg/link/blob/v1.2.0/LICENSE) |
| `github.com/pires/go-proxyproto` | v0.15.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/pires/go-proxyproto/blob/v0.15.0/LICENSE) |
| `github.com/pkg/browser` | v0.0.0-20240102092130-5ac0b6a4141c | BSD-2-Clause | Copyright (c) 2014, Dave Cheney <dave@cheney.net> | [notice](https://github.com/pkg/browser/blob/5ac0b6a4141c/LICENSE) |
| `github.com/pkg/errors` | v0.9.1 | BSD-2-Clause | Copyright (c) 2015, Dave Cheney <dave@cheney.net> | [notice](https://github.com/pkg/errors/blob/v0.9.1/LICENSE) |
| `github.com/pquerna/otp` | v1.5.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/pquerna/otp/blob/v1.5.0/LICENSE) |
| `github.com/prometheus/client_golang/internal/github.com/golang/gddo/httputil` | v1.23.2 | BSD-3-Clause | Copyright (c) 2013 The Go Authors. All rights reserved. | [notice](https://github.com/prometheus/client_golang/blob/v1.23.2/internal/github.com/golang/gddo/LICENSE) |
| `github.com/prometheus/client_golang/prometheus` | v1.23.2 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/prometheus/client_golang/blob/v1.23.2/LICENSE) |
| `github.com/prometheus/client_model/go` | v0.6.2 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/prometheus/client_model/blob/v0.6.2/LICENSE) |
| `github.com/prometheus/common` | v0.66.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/prometheus/common/blob/v0.66.1/LICENSE) |
| `github.com/prometheus/procfs` | v0.16.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/prometheus/procfs/blob/v0.16.1/LICENSE) |
| `github.com/ProtonMail/go-crypto` | v1.5.2 | BSD-3-Clause | Copyright (c) 2009 The Go Authors. All rights reserved. | [notice](https://github.com/ProtonMail/go-crypto/blob/v1.5.2/LICENSE) |
| `github.com/redis/go-redis/v9` | v9.21.0 | BSD-2-Clause | Copyright (c) 2013 The github.com/redis/go-redis Authors. | [notice](https://github.com/redis/go-redis/blob/v9.21.0/LICENSE) |
| `github.com/regfish/regfish-dnsapi-go` | v0.1.1 | MIT | Copyright (c) 2024 regfish GmbH | [notice](https://github.com/regfish/regfish-dnsapi-go/blob/v0.1.1/LICENSE) |
| `github.com/remyoudompheng/bigfft` | v0.0.0-20230129092748-24d4a6f8daec | BSD-3-Clause | Copyright (c) 2012 The Go Authors. All rights reserved. | [notice](https://github.com/remyoudompheng/bigfft/blob/24d4a6f8daec/LICENSE) |
| `github.com/sacloud/api-client-go` | v0.3.5 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/sacloud/api-client-go/blob/v0.3.5/LICENSE) |
| `github.com/sacloud/go-http` | v0.1.9 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/sacloud/go-http/blob/v0.1.9/LICENSE) |
| `github.com/sacloud/iaas-api-go` | v1.23.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/sacloud/iaas-api-go/blob/v1.23.1/LICENSE) |
| `github.com/sacloud/iaas-api-go/naked` | v1.23.1 | Apache-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/sacloud/iaas-api-go/blob/v1.23.1/naked/license.go) |
| `github.com/sacloud/packages-go` | v0.0.12 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/sacloud/packages-go/blob/v0.0.12/LICENSE) |
| `github.com/sagikazarmark/slog-shim` | v0.1.0 | BSD-3-Clause | Copyright (c) 2009 The Go Authors. All rights reserved. | [notice](https://github.com/sagikazarmark/slog-shim/blob/v0.1.0/LICENSE) |
| `github.com/scaleway/scaleway-sdk-go` | v1.0.0-beta.36 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/scaleway/scaleway-sdk-go/blob/v1.0.0-beta.36/LICENSE) |
| `github.com/selectel/domains-go/pkg/v2` | v1.1.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/selectel/domains-go/blob/v1.1.0/LICENSE) |
| `github.com/selectel/go-selvpcclient/v4/selvpcclient` | v4.2.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/selectel/go-selvpcclient/blob/v4.2.0/LICENSE) |
| `github.com/shirou/gopsutil/v4` | v4.26.9 | BSD-3-Clause | Copyright (c) 2014, WAKAYAMA Shirou | [notice](https://github.com/shirou/gopsutil/blob/v4.26.9/LICENSE) |
| `github.com/shopspring/decimal` | v1.4.0 | MIT | Copyright (c) 2015 Spring, Inc. | [notice](https://github.com/shopspring/decimal/blob/v1.4.0/LICENSE) |
| `github.com/sirupsen/logrus` | v1.9.3 | MIT | Copyright (c) 2014 Simon Eskildsen | [notice](https://github.com/sirupsen/logrus/blob/v1.9.3/LICENSE) |
| `github.com/softlayer/softlayer-go` | v1.2.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/softlayer/softlayer-go/blob/v1.2.1/LICENSE) |
| `github.com/softlayer/xmlrpc` | v0.0.0-20200409220501-5f089df7cb7e | MIT | Copyright (C) 2012 Dmitry Maksimov | [notice](https://github.com/softlayer/xmlrpc/blob/5f089df7cb7e/LICENSE) |
| `github.com/sony/gobreaker` | v1.0.0 | MIT | Copyright 2015 Sony Corporation | [notice](https://github.com/sony/gobreaker/blob/v1.0.0/LICENSE) |
| `github.com/spf13/afero` | v1.11.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/spf13/afero/blob/v1.11.0/LICENSE.txt) |
| `github.com/spf13/cast` | v1.7.0 | MIT | Copyright (c) 2014 Steve Francia | [notice](https://github.com/spf13/cast/blob/v1.7.0/LICENSE) |
| `github.com/spf13/pflag` | v1.0.7 | BSD-3-Clause | Copyright (c) 2012 Alex Ogier. All rights reserved. | [notice](https://github.com/spf13/pflag/blob/v1.0.7/LICENSE) |
| `github.com/spf13/viper` | v1.18.2 | MIT | Copyright (c) 2014 Steve Francia | [notice](https://github.com/spf13/viper/blob/v1.18.2/LICENSE) |
| `github.com/stretchr/objx` | v0.5.3 | MIT | Copyright (c) 2014 Stretchr, Inc. | [notice](https://github.com/stretchr/objx/blob/v0.5.3/LICENSE) |
| `github.com/stretchr/testify` | v1.12.1 | MIT | Copyright (c) 2012-2020 Mat Ryer, Tyler Bunnell and contributors. | [notice](https://github.com/stretchr/testify/blob/v1.12.1/LICENSE) |
| `github.com/stretchr/testify/internal/difflib` | v1.12.1 | BSD-3-Clause | Copyright (c) 2013, Patrick Mezard | [notice](https://github.com/stretchr/testify/blob/v1.12.1/internal/difflib/LICENSE) |
| `github.com/stretchr/testify/internal/spew` | v1.12.1 | ISC | Copyright (c) 2012-2016 Dave Collins <dave@davec.name> | [notice](https://github.com/stretchr/testify/blob/v1.12.1/internal/spew/LICENSE) |
| `github.com/subosito/gotenv` | v1.6.0 | MIT | Copyright (c) 2013 Alif Rachmawadi | [notice](https://github.com/subosito/gotenv/blob/v1.6.0/LICENSE) |
| `github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common` | v1.3.83 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/tencentcloud/tencentcloud-sdk-go/blob/tencentcloud/common/v1.3.83/tencentcloud/common/LICENSE) |
| `github.com/tjfoc/gmsm` | v1.4.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/tjfoc/gmsm/blob/v1.4.1/LICENSE) |
| `github.com/tklauser/go-sysconf` | v0.4.0 | BSD-3-Clause | Copyright (c) 2018-2022, Tobias Klauser | [notice](https://github.com/tklauser/go-sysconf/blob/v0.4.0/LICENSE) |
| `github.com/tklauser/numcpus` | v0.12.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/tklauser/numcpus/blob/v0.12.0/LICENSE) |
| `github.com/transip/gotransip/v6` | v6.26.2 | MIT | Copyright (c) 2018 TransIP B.V. | [notice](https://github.com/transip/gotransip/blob/v6.26.2/LICENSE) |
| `github.com/tursodatabase/libsql-client-go` | v0.0.0-20260528064733-9d5d30a29a60 | MIT | 2023-2024 Turso Authors | [notice](https://github.com/tursodatabase/libsql-client-go/blob/9d5d30a29a60/LICENSE) |
| `github.com/ucloud/ucloud-sdk-go` | v0.22.63 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/ucloud/ucloud-sdk-go/blob/v0.22.63/LICENSE) |
| `github.com/ultradns/ultradns-go-sdk` | v1.8.1-20250722213956-faef419 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/ultradns/ultradns-go-sdk/blob/v1.8.1-20250722213956-faef419/LICENSE) |
| `github.com/vinyldns/go-vinyldns/vinyldns` | v0.9.17 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/vinyldns/go-vinyldns/blob/v0.9.17/LICENSE) |
| `github.com/volcengine/volc-sdk-golang` | v1.0.242 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/volcengine/volc-sdk-golang/blob/v1.0.242/LICENSE) |
| `github.com/vultr/govultr/v3` | v3.31.0 | MIT | Copyright (c) 2019 Vultr | [notice](https://github.com/vultr/govultr/blob/v3.31.0/LICENSE) |
| `github.com/yandex-cloud/go-genproto/yandex/cloud` | v0.73.0 | MIT | Copyright (c) 2018 YANDEX LLC | [notice](https://github.com/yandex-cloud/go-genproto/blob/v0.73.0/LICENSE) |
| `github.com/yandex-cloud/go-sdk/services/dns/v1` | v0.0.54 | MIT | Copyright (c) 2018 YANDEX LLC | [notice](https://github.com/yandex-cloud/go-sdk/blob/services/dns/v0.0.54/services/dns/LICENSE) |
| `github.com/yandex-cloud/go-sdk/v2` | v2.92.0 | MIT | Copyright (c) 2018 YANDEX LLC | [notice](https://github.com/yandex-cloud/go-sdk/blob/v2.92.0/LICENSE) |
| `github.com/youmark/pkcs8` | v0.0.0-20240726163527-a2c0da244d78 | MIT | Copyright (c) 2014 youmark | [notice](https://github.com/youmark/pkcs8/blob/a2c0da244d78/LICENSE) |
| `github.com/yuin/goldmark` | v1.8.7 | MIT | Copyright (c) 2019 Yusuke Inuzuka | [notice](https://github.com/yuin/goldmark/blob/v1.8.7/LICENSE) |
| `go.mongodb.org/mongo-driver` | v1.17.9 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/mongodb/mongo-go-driver/blob/v1.17.9/LICENSE) |
| `go.opentelemetry.io/auto/sdk` | v1.2.1 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/open-telemetry/opentelemetry-go-instrumentation/blob/sdk/v1.2.1/sdk/LICENSE) |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | v0.67.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/open-telemetry/opentelemetry-go-contrib/blob/instrumentation/net/http/otelhttp/v0.67.0/instrumentation/net/http/otelhttp/LICENSE) |
| `go.opentelemetry.io/otel` | v1.44.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/open-telemetry/opentelemetry-go/blob/v1.44.0/LICENSE) |
| `go.opentelemetry.io/otel/metric` | v1.44.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/open-telemetry/opentelemetry-go/blob/metric/v1.44.0/metric/LICENSE) |
| `go.opentelemetry.io/otel/trace` | v1.44.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/open-telemetry/opentelemetry-go/blob/trace/v1.44.0/trace/LICENSE) |
| `go.uber.org/atomic` | v1.11.0 | MIT | Copyright (c) 2016 Uber Technologies, Inc. | [notice](https://github.com/uber-go/atomic/blob/v1.11.0/LICENSE.txt) |
| `go.uber.org/multierr` | v1.11.0 | MIT | Copyright (c) 2017-2021 Uber Technologies, Inc. | [notice](https://github.com/uber-go/multierr/blob/v1.11.0/LICENSE.txt) |
| `go.uber.org/ratelimit` | v0.3.1 | MIT | Copyright (c) 2016 Uber Technologies, Inc. | [notice](https://github.com/uber-go/ratelimit/blob/v0.3.1/LICENSE) |
| `go.uber.org/zap` | v1.27.0 | MIT | Copyright (c) 2016-2017 Uber Technologies, Inc. | [notice](https://github.com/uber-go/zap/blob/v1.27.0/LICENSE) |
| `go.yaml.in/yaml/v2` | v2.4.2 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/yaml/go-yaml/blob/v2.4.2/LICENSE) |
| `go.yaml.in/yaml/v3` | v3.0.5 | MIT | See linked upstream license/NOTICE | [notice](https://github.com/yaml/go-yaml/blob/v3.0.5/LICENSE) |
| `golang.org/x/crypto` | v0.57.0 | BSD-3-Clause | Copyright 2009 The Go Authors. | [notice](https://cs.opensource.google/go/x/crypto/+/v0.57.0:LICENSE) |
| `golang.org/x/mod/semver` | v0.41.0 | BSD-3-Clause | Copyright 2009 The Go Authors. | [notice](https://cs.opensource.google/go/x/mod/+/v0.41.0:LICENSE) |
| `golang.org/x/net` | v0.60.0 | BSD-3-Clause | Copyright 2009 The Go Authors. | [notice](https://cs.opensource.google/go/x/net/+/v0.60.0:LICENSE) |
| `golang.org/x/oauth2` | v0.36.0 | BSD-3-Clause | Copyright 2009 The Go Authors. | [notice](https://cs.opensource.google/go/x/oauth2/+/v0.36.0:LICENSE) |
| `golang.org/x/sync/singleflight` | v0.23.0 | BSD-3-Clause | Copyright 2009 The Go Authors. | [notice](https://cs.opensource.google/go/x/sync/+/v0.23.0:LICENSE) |
| `golang.org/x/sys` | v0.48.0 | BSD-3-Clause | Copyright 2009 The Go Authors. | [notice](https://cs.opensource.google/go/x/sys/+/v0.48.0:LICENSE) |
| `golang.org/x/term` | v0.46.0 | BSD-3-Clause | Copyright 2009 The Go Authors. | [notice](https://cs.opensource.google/go/x/term/+/v0.46.0:LICENSE) |
| `golang.org/x/text` | v0.42.0 | BSD-3-Clause | Copyright 2009 The Go Authors. | [notice](https://cs.opensource.google/go/x/text/+/v0.42.0:LICENSE) |
| `golang.org/x/time/rate` | v0.15.0 | BSD-3-Clause | Copyright 2009 The Go Authors. | [notice](https://cs.opensource.google/go/x/time/+/v0.15.0:LICENSE) |
| `google.golang.org/api` | v0.276.0 | BSD-3-Clause | Copyright (c) 2011 Google Inc. All rights reserved. | [notice](https://github.com/googleapis/google-api-go-client/blob/v0.276.0/LICENSE) |
| `google.golang.org/api/internal/third_party/uritemplates` | v0.276.0 | BSD-3-Clause | Copyright (c) 2013 Joshua Tacoma. All rights reserved. | [notice](https://github.com/googleapis/google-api-go-client/blob/v0.276.0/internal/third_party/uritemplates/LICENSE) |
| `google.golang.org/genproto/googleapis/api` | v0.0.0-20260526163538-3dc84a4a5aaa | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/googleapis/go-genproto/blob/3dc84a4a5aaa/googleapis/api/LICENSE) |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260727163830-6c54dddc4772 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/googleapis/go-genproto/blob/6c54dddc4772/googleapis/rpc/LICENSE) |
| `google.golang.org/grpc` | v1.83.2 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/grpc/grpc-go/blob/v1.83.2/LICENSE) |
| `google.golang.org/protobuf` | v1.36.11 | BSD-3-Clause | Copyright (c) 2018 The Go Authors. All rights reserved. | [notice](https://github.com/protocolbuffers/protobuf-go/blob/v1.36.11/LICENSE) |
| `gopkg.in/ini.v1` | v1.67.1 | Apache-2.0 | See linked upstream license/NOTICE | [notice](https://github.com/go-ini/ini/blob/v1.67.1/LICENSE) |
| `gopkg.in/ns1/ns1-go.v2/rest` | v2.17.2 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/ns1/ns1-go/blob/v2.17.2/LICENSE.txt) |
| `gopkg.in/yaml.v2` | v2.4.0 | Apache-2.0 | copyright notice that is included in or attached to the work | [notice](https://github.com/go-yaml/yaml/blob/v2.4.0/LICENSE) |
| `gopkg.in/yaml.v3` | v3.0.1 | MIT | copyright staring in 2011 when the project was ported over: | [notice](https://github.com/go-yaml/yaml/blob/v3.0.1/LICENSE) |
| `modernc.org/libc` | v1.74.1 | MIT | Copyright (c) 2009 The Go Authors. All rights reserved. | [notice](https://gitlab.com/cznic/libc/blob/v1.74.1/LICENSE-3RD-PARTY.md) |
| `modernc.org/mathutil` | v1.7.1 | BSD-3-Clause | 2014 The mathutil Authors | [notice](https://gitlab.com/cznic/mathutil/-/blob/master/LICENSE) |
| `modernc.org/memory` | v1.11.0 | BSD-3-Clause | Copyright (c) 2009 The Go Authors. All rights reserved. | [notice](https://gitlab.com/cznic/memory/blob/v1.11.0/LICENSE-GO) |
| `modernc.org/sqlite` | v1.54.0 | BSD-3-Clause | Copyright (c) 2017 The Sqlite Authors. All rights reserved. | [notice](https://gitlab.com/cznic/sqlite/blob/v1.54.0/LICENSE) |

#### Bubble Tea terminal UI and transitive packages

| Library/package | Version | License | Copyright/attribution | Upstream license |
|---------|---------|---------|---------|---------|
| `github.com/atotto/clipboard` | v0.1.4 | BSD-3-Clause | Copyright (c) 2013 Ato Araki. All rights reserved. | [notice](https://github.com/atotto/clipboard/blob/v0.1.4/LICENSE) |
| `github.com/aymanbagabas/go-osc52/v2` | v2.0.1 | MIT | Copyright (c) 2022 Ayman Bagabas | [notice](https://github.com/aymanbagabas/go-osc52/blob/v2.0.1/LICENSE) |
| `github.com/charmbracelet/bubbles` | v0.21.0 | MIT | Copyright (c) 2020-2023 Charmbracelet, Inc | [notice](https://github.com/charmbracelet/bubbles/blob/v0.21.0/LICENSE) |
| `github.com/charmbracelet/bubbletea` | v1.3.10 | MIT | Copyright (c) 2020-2025 Charmbracelet, Inc | [notice](https://github.com/charmbracelet/bubbletea/blob/v1.3.10/LICENSE) |
| `github.com/charmbracelet/colorprofile` | v0.2.3-0.20250311203215-f60798e515dc | MIT | Copyright (c) 2020-2024 Charmbracelet, Inc | [notice](https://github.com/charmbracelet/colorprofile/blob/f60798e515dc/LICENSE) |
| `github.com/charmbracelet/lipgloss` | v1.1.0 | MIT | Copyright (c) 2021-2023 Charmbracelet, Inc | [notice](https://github.com/charmbracelet/lipgloss/blob/v1.1.0/LICENSE) |
| `github.com/charmbracelet/x/ansi` | v0.10.1 | MIT | Copyright (c) 2023 Charmbracelet, Inc. | [notice](https://github.com/charmbracelet/x/blob/ansi/v0.10.1/ansi/LICENSE) |
| `github.com/charmbracelet/x/cellbuf` | v0.0.13-0.20250311204145-2c3ea96c31dd | MIT | Copyright (c) 2023 Charmbracelet, Inc. | [notice](https://github.com/charmbracelet/x/blob/2c3ea96c31dd/cellbuf/LICENSE) |
| `github.com/charmbracelet/x/term` | v0.2.1 | MIT | Copyright (c) 2023 Charmbracelet, Inc. | [notice](https://github.com/charmbracelet/x/blob/term/v0.2.1/term/LICENSE) |
| `github.com/lucasb-eyer/go-colorful` | v1.2.0 | MIT | Copyright (c) 2013 Lucas Beyer | [notice](https://github.com/lucasb-eyer/go-colorful/blob/v1.2.0/LICENSE) |
| `github.com/mattn/go-isatty` | v0.0.21 | MIT | Copyright (c) Yasuhiro MATSUMOTO | [notice](https://github.com/mattn/go-isatty/blob/v0.0.21/LICENSE) |
| `github.com/mattn/go-runewidth` | v0.0.16 | MIT | Copyright (c) 2016 Yasuhiro Matsumoto | [notice](https://github.com/mattn/go-runewidth/blob/v0.0.16/LICENSE) |
| `github.com/muesli/ansi` | v0.0.0-20230316100256-276c6243b2f6 | MIT | Copyright (c) 2021 Christian Muehlhaeuser | [notice](https://github.com/muesli/ansi/blob/276c6243b2f6/LICENSE) |
| `github.com/muesli/cancelreader` | v0.2.2 | MIT | Copyright (c) 2022 Erik Geiser and Christian Muehlhaeuser | [notice](https://github.com/muesli/cancelreader/blob/v0.2.2/LICENSE) |
| `github.com/muesli/termenv` | v0.16.0 | MIT | Copyright (c) 2019 Christian Muehlhaeuser | [notice](https://github.com/muesli/termenv/blob/v0.16.0/LICENSE) |
| `github.com/rivo/uniseg` | v0.4.7 | MIT | Copyright (c) 2019 Oliver Kuederle | [notice](https://github.com/rivo/uniseg/blob/v0.4.7/LICENSE.txt) |
| `github.com/xo/terminfo` | v0.0.0-20220910002029-abceb7e1c41e | MIT | Copyright (c) 2016 Anmol Sethi | [notice](https://github.com/xo/terminfo/blob/abceb7e1c41e/LICENSE) |

#### Optional native GUI and graphics packages

| Library/package | Version | License | Copyright/attribution | Upstream license |
|---------|---------|---------|---------|---------|
| `github.com/coregx/signals` | v0.1.1 | MIT | Copyright (c) 2025 Andy Goryachev | [notice](https://github.com/coregx/signals/blob/v0.1.1/LICENSE) |
| `github.com/go-webgpu/goffi` | v0.6.3 | MIT | Copyright (c) 2025-2026 Andrey Kolkov and GoGPU Contributors | [notice](https://github.com/go-webgpu/goffi/blob/v0.6.3/LICENSE) |
| `github.com/gogpu/gg` | v0.52.5 | MIT | Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors | [notice](https://github.com/gogpu/gg/blob/v0.52.5/LICENSE) |
| `github.com/gogpu/gogpu` | v0.53.0 | MIT | Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors | [notice](https://github.com/gogpu/gogpu/blob/v0.53.0/LICENSE) |
| `github.com/gogpu/gpucontext` | v0.28.0 | MIT | Copyright (c) 2026 Andrey Kolkov and GoGPU Contributors | [notice](https://github.com/gogpu/gpucontext/blob/v0.28.0/LICENSE) |
| `github.com/gogpu/gputypes` | v0.5.2 | MIT | Copyright (c) 2026 Andrey Kolkov and GoGPU Contributors | [notice](https://github.com/gogpu/gputypes/blob/v0.5.2/LICENSE) |
| `github.com/gogpu/naga` | v0.18.0 | MIT | Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors | [notice](https://github.com/gogpu/naga/blob/v0.18.0/LICENSE) |
| `github.com/gogpu/ui` | v0.1.54 | MIT | Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors | [notice](https://github.com/gogpu/ui/blob/v0.1.54/LICENSE) |
| `github.com/gogpu/wgpu` | v0.31.6 | MIT | Copyright (c) 2025 Andrey Kolkov and GoGPU Contributors | [notice](https://github.com/gogpu/wgpu/blob/v0.31.6/LICENSE) |
| `golang.org/x/image` | v0.45.0 | BSD-3-Clause | Copyright 2009 The Go Authors. | [notice](https://cs.opensource.google/go/x/image/+/v0.45.0:LICENSE) |

For BSD-3-Clause libraries, the non-endorsement clause applies: neither the
name of the copyright holder nor the names of its contributors may be used to
endorse or promote products derived from this software without specific prior
written permission.

Full license texts available at: https://spdx.org/licenses/


## Apache License Notices

### `github.com/Azure/azure-sdk-for-go/NOTICE.txt`

NOTICES AND INFORMATION
Do Not Translate or Localize

This software incorporates material from third parties. Microsoft makes certain
open source code available at https://3rdpartysource.microsoft.com, or you may
send a check or money order for US $5.00, including the product name, the open
source component name, and version number, to:

Source Code Compliance Team
Microsoft Corporation
One Microsoft Way
Redmond, WA 98052
USA

Notwithstanding any other terms, you may reverse engineer this software to the
extent required to debug changes to any libraries licensed under the GNU Lesser
General Public License.

------------------------------------------------------------------------------

Azure SDK for Go uses third-party libraries or other resources that may be
distributed under licenses different than the Azure SDK for Go software.

In the event that we accidentally failed to list a required notice, please
bring it to our attention. Post an issue or email us:

           azgosdkhelp@microsoft.com

The attached notices are provided for information only.

### `github.com/aws/aws-sdk-go-v2/NOTICE.txt`

AWS SDK for Go
Copyright 2015 Amazon.com, Inc. or its affiliates. All Rights Reserved.
Copyright 2014-2015 Stripe, Inc.

### `github.com/aws/smithy-go/NOTICE`

Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

### `github.com/pquerna/otp/NOTICE`

otp
Copyright (c) 2014, Paul Querna

This product includes software developed by
Paul Querna (http://paul.querna.org/).

### `github.com/prometheus/client_golang/prometheus/NOTICE`

Prometheus instrumentation library for Go applications
Copyright 2012-2015 The Prometheus Authors

This product includes software developed at
SoundCloud Ltd. (http://soundcloud.com/).


The following components are included in this product:

perks - a fork of https://github.com/bmizerany/perks
https://github.com/beorn7/perks
Copyright 2013-2015 Blake Mizerany, Björn Rabenstein
See https://github.com/beorn7/perks/blob/master/README.md for license details.

Go support for Protocol Buffers - Google's data interchange format
http://github.com/golang/protobuf/
Copyright 2010 The Go Authors
See source code for license details.

### `github.com/prometheus/client_model/go/NOTICE`

Data model artifacts for Prometheus.
Copyright 2012-2015 The Prometheus Authors

This product includes software developed at
SoundCloud Ltd. (http://soundcloud.com/).

### `github.com/prometheus/common/NOTICE`

Common libraries shared by Prometheus Go components.
Copyright 2015 The Prometheus Authors

This product includes software developed at
SoundCloud Ltd. (http://soundcloud.com/).

### `github.com/prometheus/procfs/NOTICE`

procfs provides functions to retrieve system, kernel and process
metrics from the pseudo-filesystem proc.

Copyright 2014-2015 The Prometheus Authors

This product includes software developed at
SoundCloud Ltd. (http://soundcloud.com/).

### `github.com/vinyldns/go-vinyldns/vinyldns/NOTICE`

go-vinyldns

Copyright 2018 Comcast Cable Communications Management, LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This product includes software developed at Comcast (http://www.comcast.com/).

### `go.yaml.in/yaml/v2/NOTICE`

Copyright 2011-2016 Canonical Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

### `google.golang.org/grpc/NOTICE.txt`

Copyright 2014 gRPC authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

### `gopkg.in/yaml.v2/NOTICE`

Copyright 2011-2016 Canonical Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

### `gopkg.in/yaml.v3/NOTICE`

Copyright 2011-2016 Canonical Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
