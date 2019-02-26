package main

import (
	"context"
	"github.com/pborman/uuid"
	"gopkg.in/btrdb.v4"
	"log"
)

func main() {
	addr := "127.0.0.1:4410"
	ctx := context.Background()

	//setup btrdb connection
	conn, err := btrdb.Connect(ctx, addr)
	if err != nil {
		log.Fatal(err)
	}

	// query brick model for uuids
	uuids := []string{
		"d3489cfa-93a5-37e7-a274-0f35cf17b782",
		"f8017406-28c8-320d-9fe6-ba8c9ce94b09",
		"b315ed38-fae1-31ae-a53c-9202aa0ef600",
		"9395a1fb-3766-3854-b484-5b6e285c1156",
		"be0a6276-17b4-31a7-bef7-ae6b88d3440b",
		"891a1c36-ae4c-3142-a07d-d3b5d3fba271",
		"777faf39-c059-3a2d-b85b-ac3dfb12e54c",
		"96160505-25a9-399b-95db-b972d68bf362",
		"c2c477b4-c02c-32cc-9334-81fe419d0e81",
		"789579a2-c37e-3079-b04f-bc32effbb4ef",
		"7d907c16-4851-34be-93a7-46182575fa85",
		"b849d90a-85ec-3c97-8ad0-b0d76f56e6b2",
		"497145c4-2087-3e89-9c8f-f54b87dfb016",
		"f62ed54b-7b9e-319b-9b9c-9901c680b983",
		"4c910f33-cd1a-3f1d-a740-48c42e20a28b",
		"a332bccc-9516-3175-beaf-8a478b0c49b0",
		"4e4f0b0f-9918-356b-9b68-fdc49f75867d",
		"5dd8b2fa-acff-30ef-b3a6-a0ab3bc054ca",
		"148814c5-e766-3632-b54c-344ff1e469fd",
		"92ccbcb0-84ea-38de-b94a-c89209777ab0",
		"c0623266-12b8-337f-8b37-6bf40e60cfa3",
		"e6cfed6a-52e2-389c-9138-3da02794bd52",
		"d7cbcd57-d7c0-386c-a1e7-12766f05f2b3",
		"70e64713-fd4a-3f94-ad7c-ca2f83a74382",
		"989ed503-9aa2-310e-abd3-464f6169e175",
		"ecce4b33-e4d9-3b87-9e3c-1c93d4c62c08",
		"e962047c-c314-3e1b-9cc7-0c33d0d18b55",
		"11996d2f-fe5c-3838-9ae0-727b0d9d30b7",
		"bdbbbf7c-21c3-3b61-8469-8516a8c666dd",
		"c377a8a4-6905-3479-b355-eade8d62948e",
		"6e5d4f59-78d3-3085-98e3-f8d46e365c04",
		"00b88162-0468-34c2-b625-f5db61cc4e7b",
		"c6adf801-47df-395d-b8a9-9959f8ab972e",
		"6b54bf39-ffbf-30d9-90e5-76f92cba72d0",
		"e6c1a17d-375d-3018-908b-75b607de190e",
		"23194baa-a7bf-331e-98cf-33d83c1a0a5f",
		"242cde13-7d25-306a-bfc8-918ad7bb26d6",
		"a6624658-7cdc-3a9e-a084-f536a493a752",
		"f1baf473-8578-340d-8a8c-96482dc3f0b2",
		"93880771-418a-349f-b1bd-d7942641b674",
		"23d2afce-4c86-37c5-b1c2-f258758c3263",
		"15881ebd-d695-33ac-b1a7-0a466509977a",
		"2f6cd75c-0a40-3339-8af9-a68b360caa32",
		"20925ac9-61a2-3ff6-9415-78dc9d56cefc",
		"e448db62-f95a-386e-a8f0-30085a46a03e",
		"45ea5b21-9d09-359f-a80c-2bb5754f7828",
		"24f521b8-eef6-3ca8-9c71-59752b0ba45d",
		"28640fdc-6012-3e41-a759-208c29440ded",
		"d919a14e-3ebd-3e5b-9727-0df54f287032",
		"fa777824-6473-3004-a705-eec445eb9106",
		"83ab0b1a-0a96-3530-bb66-f0f092e4a517",
		"b4c40ab6-d337-3123-b914-5c89dd4a15b1",
		"47e37419-5091-3877-a476-bb6f942e93af",
		"8de1a7e2-64f5-3e0a-a5f1-edd9220037f4",
		"9d99692f-c79b-3325-831d-938aedf432b0",
		"9ba535a0-2c4a-3005-9b20-ae5b9c48a5a8",
		"cab606d0-6d14-3aff-a0c2-e862ae297f22",
		"522cb9e7-511a-3c8e-8ce4-347053609de2",
		"1b6c0f3b-6f52-3736-8583-5751a6407d0b",
		"6aade260-38c1-333c-8a70-fb780e796307",
		"d5522fbc-f028-3019-bee2-61367807e178",
		"61cc3aea-b039-3356-9035-97b99b00670e",
		"38b41c78-3769-330e-be0a-1f88d5ad5196",
		"06075dc0-32d8-3771-9e23-9e1254c81409",
		"16108a4b-4aec-33f1-8780-8f765e798435",
		"0c280eea-5074-3879-928d-87524a436fdd",
		"7a0f4fc8-85cd-353f-b5f8-4d40e8cd8074",
		"ab4c1735-02e1-342d-b6c3-f395dd8e3012",
		"2304ba4f-db2c-31f4-bd12-95411152045f",
		"4d6e251a-48e1-3bc0-907d-7d5440c34bb9",
		"7b49f7f2-f142-38a0-b284-42b0aef95b64",
		"f510c11b-3a04-3518-b757-7940555abb1d",
		"5e55525e-f799-3b7b-8520-8e42730946df",
		"a73c1b67-142f-3b45-baf8-e308619b6bbc",
		"4a939b52-73b5-3016-95d7-34fd1ea1d41f",
		"9a3d08a7-0489-3b9e-981b-2e2e916cd783",
		"3eaf5926-11a8-3b7c-abdb-d1b06aca2cb6",
		"a3beea1c-65e3-38e1-8710-9fd1d9605caa",
		"5cbf1af8-60ba-3e36-9ed6-b80feb4acae2",
		"00327584-54b2-35a8-aaed-182747a5dda7",
		"98180ac6-3a45-3884-afff-0c0341a9b9f1",
		"fe8d0d04-f8d2-37cc-a582-3978f3699474",
		"0d037818-02c2-3e5b-87e9-94570d43b418",
		"4a471518-3cdd-3039-ab20-9f0f964e95c0",
		"6cbee2ae-06e7-3fc3-a2fc-698fa3deadee",
		"22e4f56b-1147-3302-83df-c7631b9b0b79",
		"67b3beeb-5a75-338b-b51e-7734781fac02",
		"187ed9b8-ee9b-3042-875e-088a08da37ae",
		"c05385e5-a947-37a3-902e-f6ea45a43fe8",
		"d38446d4-32cc-34bd-b293-0a3871a6759b",
		"49cb0dff-b571-3155-b08d-3e7c9aa8a934",
		"e4d39723-5907-35bd-a9b2-fc57b58b3779",
		"3bc7ce21-2384-3ebe-aedd-8c2822b5a10c",
		"388512db-58c4-33ab-93dc-f85abcaf78d4",
		"7e543d07-16d1-32bb-94af-95a01f4675f9",
		"b47ba370-bceb-39cf-9552-d1225d910039",
		"9fa56ac1-0f8a-3ad2-86e8-72e816b875ad",
		"f1bb9994-4b72-3112-b4a6-e2eb49bd2b6f",
		"e4e0db0b-1c15-330e-a864-011e558f542e",
		"c74200d0-0ee1-3951-a7f6-2bc8857cb303",
		"04a4ac49-a888-38c6-b000-e81e3ff5e630",
		"5e55e5b1-007b-39fa-98b6-ae01baa6dccd",
		"c7e33fa6-f683-36e9-b97a-7f096e4b57d4",
		"dbbf4a91-107a-3b15-b2c0-a49b54116daa",
		"5c965d7d-e1b4-39a5-b2d8-e3e6cad0e136",
		"eeadc8ed-6255-320d-b845-84f44748fe95",
		"a4639323-1e57-3512-83fb-b01234378fd8",
		"25008947-b7db-32d4-87c1-b93be6dc5097",
		"dfb2b403-fd08-3e9b-bf3f-18c699ce40d6",
		"03099008-5224-3b61-b07e-eee445e64620",
		"c6436c6f-aeed-3b93-ae79-ee8fc910ceba",
		"e3ef6b5e-6e43-3356-80b5-74ccbd0e1f02",
		"f3d28867-362b-3f1c-a03e-754e36664a08",
		"a95899b7-5a21-3602-835e-2ed1cdf29ba1",
		"7c8acc1f-03e9-30b3-b2aa-c7ab25700076",
		"35c4245a-8ece-3d8d-aab7-599bba61538e",
		"214aa0bd-459a-3395-bb31-dd8cb07033a6",
		"4ba8333d-74cc-39f2-8938-ead227ad0f95",
		"99870724-722d-3dd6-8a44-138c6c6243af",
		"7749cd62-333a-35ed-b0d3-ca248b15c360",
		"0c51371e-6029-3302-984d-b71d850e899a",
		"e8da2b81-d179-3611-8ac7-4a1619c1407b",
		"191ec5e3-2a12-39e8-8416-212eea0eedd0",
		"33ed7b93-605c-39f4-85b9-e43b3fcdfa7b",
		"c5fefca9-58b3-37b5-ba10-ac178449d9d4",
		"e53d51a3-bbe1-3af3-9cf2-649c9ead299b",
		"5ac07704-af4b-3fdb-8038-6f464ce18c35",
		"c5c295d9-76e0-34ef-92d5-149fa25f22a3",
		"862089d9-4c06-3ac1-bc1d-84d8d8f122d8",
		"05dcc4ae-de8c-3c6d-943b-153bebb06e39",
		"f91e8678-47a2-3da6-9eec-753f4364044c",
		"ce6ea005-9d70-30e5-83a5-13c312c40aa7",
		"05240142-25e2-33f5-a3f3-bb52f2d934c5",
		"6125c4aa-7685-390d-a745-27a19e05ada2",
		"f3262678-43d5-39ab-ab42-13abd3c952f9",
		"7fc65a1a-70da-320d-923d-7d33df77e83c",
		"80198599-4d12-3692-8068-e3b9bdd7d79f",
		"6db3c938-e7b5-362e-8ec7-fd1715bfa4b6",
		"f5994eac-9691-36a6-8610-599aa3ecbd36",
		"41016955-fdb1-340e-9d38-f580a2e34800",
		"611ac5e9-33a5-3273-9fa6-548d3ad59481",
		"5b812bbc-f1c3-3d43-a655-40a3d44837fc",
		"2b3f9905-3bb7-302a-9545-7bfe2eb63547",
		"c5d201cf-fb7e-3e83-aab3-517e93df1919",
		"2f5c888d-c0c6-33b4-9c30-d609f1e16fea",
		"38b1ad08-2c56-3462-87bf-91800a1f3af1",
		"afed35be-ef70-341a-8bf4-565d815bfd90",
		"65b682ce-3a1f-3072-b64a-3a354be841c0",
		"aefa14ef-a13a-355b-9b87-51511db7f541",
		"72ef9222-40c4-3914-8e9f-6e050cb9e8fc",
		"c27adbad-b1b5-342c-91eb-a6709d365871",
		"6537ddc4-2796-3091-876d-554f9bf3ef5a",
		"97ba99af-869b-3c43-94e3-93040c9bdd33",
		"c7713607-1736-30c5-aded-8e848eb42a6a",
		"00f45f05-08f9-3def-a977-357493fad61e",
		"3dc5bd0b-d0b9-3a3e-845c-d3bc4f8df220",
		"3cea0629-6fcd-3c6a-9786-7b571eaaf13d",
	}

	for _, uustr := range uuids {
		uu := uuid.Parse(uustr)
		stream := conn.StreamFromUUID(uu)
		exists, existsErr := stream.Exists(ctx)
		if existsErr != nil {
			e := btrdb.ToCodedError(existsErr)
			if e.Code != 501 && !exists {
				log.Fatal(existsErr)
			}
		}
		tags := map[string]string{
			"name": uustr,
		}
		if !exists {
			stream, err = conn.Create(ctx, uu, "testme", tags, nil)
			if err != nil {
				log.Fatal(err)
			}
		}

		numreadings := 1000
		gentime := func(i int) int64 {
			return int64(i)
		}
		genval := func(i int) float64 {
			return float64(i)
		}
		if err := stream.InsertF(ctx, numreadings, gentime, genval); err != nil {
			log.Fatal(err)
		}

	}

}
